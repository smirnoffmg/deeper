# Deeper

Deeper starts from one thing you already know — an email, a username, a domain, a company name — and grows it into a graph of everything that's provably connected to it. The hard part is not finding things. It's knowing which of them are true.

## One scan, start to finish

The easiest way to show what that means is to point Deeper at its own author.

### A handle

```bash
deeper scan smirnoffmg
```

Within a minute Deeper has checked the handle against about 480 sites and found it registered on a few dozen of them: Hugging Face, Docker Hub, TryHackMe, dev.to, Spotify, a Russian cycling forum, an airliner-spotting site. It has also found `github.com/smirnoffmg`, with a profile, two SSH keys and fifty-odd repositories behind it.

How many of those accounts belong to the same person? From the handle alone, nobody can say. A site answering "200 OK" for `/user/smirnoffmg` proves that *someone* took that name there, nothing more.

### The traps

Earlier versions of Deeper trusted what they found and kept digging. Every one of these happened on this very scan:

- **Bots became people.** Commit history named `dependabot[bot]` and `github-actions[bot]` as authors. Deeper treated them as usernames, swept them across every site it knows, and dutifully reported a "profile" for `dependabot[bot]` on Apple's discussion forums.
- **Co-authors became aliases.** Commits carried `Co-authored-by: Claude Opus 4.8` and `Co-authored-by: Cursor`. Those name someone *else* who helped write a commit, but they came out looking like more names for the target.
- **The platform became a name.** Commits made through GitHub's web editor are signed by `GitHub <noreply@github.com>`, so "GitHub" showed up as a person.
- **A namesake became the target.** `github.com/<handle>` belongs to whoever registered it, not necessarily to the person who uses that handle elsewhere. One unlucky collision is enough to pull a stranger's employer, company registry entries and domains into the graph.

None of these is a bug in a single plugin. They are the same mistake every time: treating a guess as a fact, and then building on it.

### A guess is not a fact

So every edge in Deeper's graph now says how much it proves, and why:

| Strength | Meaning | Example basis |
|---|---|---|
| `proof` | the source binds the two by itself | the seed you typed, a DNS answer |
| `strong` | an authoritative source asserts it | `commit author`, `repos of github.com/<login>` |
| `weak` | existence or resemblance only | `sherlock status_code check`, `namesake: github.com/<login> exists`, `co-authored-by trailer` |

A weak finding is recorded, never thrown away, but Deeper looks at it for exactly one more step and stops. The swept accounts are fetched and whatever their profiles say is kept, but none of it is followed further. Bots and the web-flow committer are dropped outright, because they are not people at all. In the HTML report weak edges are dashed, and hovering one tells you why it is there: `via SocialProfilesPlugin · weak: sherlock status_code check`.

For the bare handle that leaves 93 traces, and only two of them are expanded: the handle itself and the GitHub account it *might* be.

### Telling Deeper what you know

If you know which account is the target's, say so:

```bash
deeper scan https://github.com/smirnoffmg
```

A seed is `proof`, so this time the whole chain opens. The account's repositories lead to commits, commits lead to the names and emails their author has used over the years, those emails lead to a Gravatar profile, and so on: 106 traces joined by 289 edges, of which 249 are strong, 39 weak and one, the seed, proof. The AI co-authors are still there, as weak edges with their reason attached. The bots are gone.

### And a domain

```bash
deeper scan smirnoffmg.dev
```

Infrastructure is the easy case: DNS answers, certificate transparency logs and IP registries are authoritative about what they report. The scan finds Cloudflare nameservers, a CNAME to GitHub Pages and the GitHub CDN addresses behind it, the CAA records, the `www` host from the certificate logs: 34 traces, every edge strong.

Strong is not the same as interesting, though. The zone's SOA record names `dns@cloudflare.com`, which is a true fact about the zone and says nothing about its owner. Telling relevant from merely true is the next problem, not one Deeper solves yet.

## How it works

Everything Deeper knows is a **trace**: a single typed fact such as an email address, a domain, an IP, a GitHub account, a company name or a key. Each trace goes to every plugin that knows what to do with that kind of fact, and whatever they find becomes new traces, breadth-first, until the graph stops growing. Every edge records its parent, the plugin that found it, its strength and its basis, so any fact can be traced back to how Deeper got there.

- **Nothing is asked twice.** Every (fact, source) pair is checked once and remembered, so a re-run doesn't repeat work or hammer the same site.
- **No source gets hit harder than it can take.** Requests to each destination are paced independently, so one noisy plugin can't get the whole investigation rate-limited.

What it can reach from a single identifier:

- **Identity**: the real name, company and role behind a username or email; company registries; academic publications.
- **Social presence**: a broad existence sweep across hundreds of platforms, plus focused plugins for the platforms worth reading properly, including ones with cryptographically verified links between accounts.
- **Infrastructure**: domains, subdomains, DNS records, IP ranges and hosting, via certificate transparency, DNS and WHOIS.
- **Code footprint**: repositories, commit history and the names and emails behind commit authorship.
- **Contact surface**: emails from crawled pages and commits, and the SSH and PGP keys published on a code-hosting profile.

Architecture and performance-tuning notes live in [`docs/`](docs/), and design decisions in [`spec/`](spec/).

## Getting started

```bash
git clone https://github.com/smirnoffmg/deeper.git
cd deeper
make build
./build/deeper scan <email | username | domain | company | https://github.com/<login>>
```

Set `DEEPER_GITHUB_TOKEN` to a GitHub token. Without it, GitHub's anonymous limit of 60 requests an hour runs out within a scan or two.

Every scan renders its graph to a self-contained, interactive HTML report (`~/.deeper/reports/scan-<id>.html`) and, for the default table output, opens it in your browser: pan, zoom, hover for details, click a trace to isolate its neighbors. `--no-open` skips the browser without losing the report.

## MCP server

`mcpd` exposes Deeper to MCP clients and AI agents, so an assistant can run scans and inspect the plugin catalog as tools. Build it with `make build-mcpd` (binary at `./build/mcpd`).

By default it runs as a daemon serving the MCP **Streamable HTTP** transport at `/mcp`, so clients connect by URL:

```bash
./build/mcpd                 # listens on http://127.0.0.1:8765/mcp
./build/mcpd -addr :9000     # change the bind address
./build/mcpd -stdio          # use the stdio transport instead (client spawns it as a subprocess)
```

It binds to loopback by default on purpose: the server can trigger scans — `active_scan` contacts targets directly — so it must not be reachable from the network unless you deliberately put authentication in front of it.

It registers three tools:

- `scan` — passive OSINT scan of a seed identifier; queries third-party sources only and never contacts the target.
- `active_scan` — active-recon scan that additionally contacts the target directly (TCP port scan, directory brute force); only use it against targets you are authorized to actively probe.
- `list_plugins` — list registered plugins grouped by trace type.

## Responsible use

Deeper is built for legitimate OSINT research and authorized security testing. Make sure you have proper authorization before pointing it at any person, system, or organization you don't own. The examples above are the author's own accounts.

By default a scan is **passive** — it only queries third-party sources (certificate logs, WHOIS, DNS, code-host APIs, archives) and never touches the target directly. Active-recon plugins that do contact the target — a TCP port scan, directory brute-forcing — run only when you pass `--active` (or set `DEEPER_ACTIVE_SCAN=true`). Use them only where your authorization covers active probing.

## License

MIT — see [LICENSE](LICENSE).
