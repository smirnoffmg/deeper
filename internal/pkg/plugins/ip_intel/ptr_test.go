package ip_intel

import (
	"context"
	"errors"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAddrLookup struct {
	names []string
	err   error
}

func (f *fakeAddrLookup) LookupAddr(ctx context.Context, addr string) ([]string, error) {
	return f.names, f.err
}

func TestLookupPTR_KnownHostnames(t *testing.T) {
	lookups := &fakeAddrLookup{
		names: []string{
			"static.198-51-100-5.clients.your-server.de.",
		},
	}

	traces := lookupPTR(context.Background(), "198.51.100.5", lookups)

	require.Len(t, traces, 1)
	assert.Equal(t, entities.DnsRecordPTR, traces[0].Type)
	assert.Equal(t, "static.198-51-100-5.clients.your-server.de.", traces[0].Value)
}

func TestLookupPTR_MultipleHostnames(t *testing.T) {
	lookups := &fakeAddrLookup{
		names: []string{"host1.example.com.", "host2.example.com."},
	}

	traces := lookupPTR(context.Background(), "198.51.100.5", lookups)

	require.Len(t, traces, 2)
	assert.Equal(t, entities.DnsRecordPTR, traces[0].Type)
	assert.Equal(t, entities.DnsRecordPTR, traces[1].Type)
}

func TestLookupPTR_LookupError(t *testing.T) {
	lookups := &fakeAddrLookup{
		err: errors.New("no PTR record"),
	}

	traces := lookupPTR(context.Background(), "198.51.100.5", lookups)

	assert.Empty(t, traces)
}

func TestLookupPTR_EmptyResult(t *testing.T) {
	lookups := &fakeAddrLookup{
		names: []string{},
	}

	traces := lookupPTR(context.Background(), "198.51.100.5", lookups)

	assert.Empty(t, traces)
}
