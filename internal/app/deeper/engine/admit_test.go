package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

var admitSeed = entities.Trace{Value: "seed", Type: entities.Username}

func tr(v string) entities.Trace {
	return entities.Trace{Value: v, Type: entities.Username}
}

func edge(parent, child entities.Trace, s entities.Strength) entities.Discovery {
	return entities.Discovery{Parent: parent, PluginName: "p", Child: child, Strength: s}
}

func newAdmitState() (map[entities.Trace]*nodeState, map[entities.Trace][]entities.Discovery) {
	nodes := map[entities.Trace]*nodeState{admitSeed: {strength: entities.StrengthProof, queued: true}}
	return nodes, map[entities.Trace][]entities.Discovery{}
}

func TestAdmit_WeakEdgeFromProofSeedIsWeakAndEnqueued(t *testing.T) {
	nodes, children := newAdmitState()
	c := tr("c")

	got := admit(nodes, children, edge(admitSeed, c, entities.StrengthWeak))

	assert.Equal(t, []entities.Trace{c}, got)
	assert.Equal(t, nodeState{strength: entities.StrengthWeak, queued: true}, *nodes[c])
}

func TestAdmit_EdgeFromWeakParentRecordedNotEnqueued(t *testing.T) {
	for _, s := range []entities.Strength{entities.StrengthWeak, entities.StrengthStrong, entities.StrengthProof} {
		t.Run(string(s), func(t *testing.T) {
			nodes, children := newAdmitState()
			w, c := tr("w"), tr("c")
			admit(nodes, children, edge(admitSeed, w, entities.StrengthWeak))

			got := admit(nodes, children, edge(w, c, s))

			assert.Empty(t, got)
			assert.Equal(t, nodeState{strength: entities.StrengthWeak, queued: false}, *nodes[c])
			assert.Len(t, children[w], 1)
		})
	}
}

func TestAdmit_WeakThenStrongPromotesAndEnqueues(t *testing.T) {
	nodes, children := newAdmitState()
	w, p, c := tr("w"), tr("p"), tr("c")
	admit(nodes, children, edge(admitSeed, w, entities.StrengthWeak))
	admit(nodes, children, edge(admitSeed, p, entities.StrengthStrong))
	assert.Empty(t, admit(nodes, children, edge(w, c, entities.StrengthStrong)))

	got := admit(nodes, children, edge(p, c, entities.StrengthStrong))

	assert.Equal(t, []entities.Trace{c}, got)
	assert.Equal(t, nodeState{strength: entities.StrengthStrong, queued: true}, *nodes[c])
}

func TestAdmit_SameStrengthFromNonWeakParentEnqueuesInEitherOrder(t *testing.T) {
	w, p, c := tr("w"), tr("p"), tr("c")
	fromWeak := edge(w, c, entities.StrengthWeak)
	fromStrong := edge(p, c, entities.StrengthWeak)

	run := func(order ...entities.Discovery) (nodeState, []entities.Trace) {
		nodes, children := newAdmitState()
		admit(nodes, children, edge(admitSeed, w, entities.StrengthWeak))
		admit(nodes, children, edge(admitSeed, p, entities.StrengthStrong))
		var enqueued []entities.Trace
		for _, d := range order {
			enqueued = append(enqueued, admit(nodes, children, d)...)
		}
		return *nodes[c], enqueued
	}

	weakFirst, enqWeakFirst := run(fromWeak, fromStrong)
	strongFirst, enqStrongFirst := run(fromStrong, fromWeak)

	assert.Equal(t, nodeState{strength: entities.StrengthWeak, queued: true}, weakFirst)
	assert.Equal(t, weakFirst, strongFirst)
	assert.Equal(t, []entities.Trace{c}, enqWeakFirst)
	assert.Equal(t, []entities.Trace{c}, enqStrongFirst)
}

func TestAdmit_PromotionRepropagatesToRecordedChildren(t *testing.T) {
	nodes, children := newAdmitState()
	w, p, g, h := tr("w"), tr("p"), tr("g"), tr("h")
	admit(nodes, children, edge(admitSeed, w, entities.StrengthWeak))
	admit(nodes, children, edge(admitSeed, p, entities.StrengthStrong))
	assert.Empty(t, admit(nodes, children, edge(w, g, entities.StrengthWeak)))
	assert.Empty(t, admit(nodes, children, edge(w, h, entities.StrengthStrong)))

	got := admit(nodes, children, edge(p, w, entities.StrengthStrong))

	assert.ElementsMatch(t, []entities.Trace{g, h}, got)
	assert.Equal(t, nodeState{strength: entities.StrengthStrong, queued: true}, *nodes[w])
	assert.Equal(t, nodeState{strength: entities.StrengthWeak, queued: true}, *nodes[g])
	assert.Equal(t, nodeState{strength: entities.StrengthStrong, queued: true}, *nodes[h])
	assert.Len(t, children[w], 2, "re-propagation must not duplicate recorded discoveries")
}

func TestAdmit_MixedStrengthCycleTerminates(t *testing.T) {
	nodes, children := newAdmitState()
	a, b := tr("a"), tr("b")
	admit(nodes, children, edge(admitSeed, a, entities.StrengthWeak))
	admit(nodes, children, edge(a, b, entities.StrengthStrong))
	admit(nodes, children, edge(b, a, entities.StrengthProof))

	got := admit(nodes, children, edge(admitSeed, b, entities.StrengthStrong))

	assert.Equal(t, []entities.Trace{b}, got)
	assert.Equal(t, nodeState{strength: entities.StrengthStrong, queued: true}, *nodes[a])
	assert.Equal(t, nodeState{strength: entities.StrengthStrong, queued: true}, *nodes[b])
}

func TestRank_OrdersWeakStrongProof(t *testing.T) {
	assert.Less(t, rank(entities.StrengthWeak), rank(entities.StrengthStrong))
	assert.Less(t, rank(entities.StrengthStrong), rank(entities.StrengthProof))
}
