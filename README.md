# Deeper

Deeper starts from one thing you already know — an email, a username, a domain, a company name — and grows it into a graph of everything that's provably connected to it.

## How it thinks

Everything Deeper knows is a **trace** — a single fact, typed and dated: an email address, a domain, an IP, a GitHub handle, a company name, a cryptographic key. Deeper never stores an opinion, only facts it can point back to a source for.

Each trace gets handed to every plugin that knows how to do something useful with that *kind* of fact. A domain trace goes to the plugins that know how to look up WHOIS records, enumerate subdomains, or check certificate transparency logs. A username trace goes to the plugins that check code-hosting platforms, developer communities, and social networks. Whatever new facts those plugins turn up become new traces, which get handed to the plugins that know what to do with *them* — and so on, breadth-first, until the graph stops growing.

A few things this approach is deliberately strict about:

- **Nothing is asked twice.** Every (fact, source) pair is checked once and remembered, so re-running an investigation doesn't repeat work or hammer the same site again.
- **No source gets hit harder than it can take.** Requests to each destination are paced independently, so one noisy plugin can't drown out the rest or get the whole investigation rate-limited.
- **A guess is not a fact.** Where a source only tells you "this account might exist" (most social-network username checks fall into this bucket), Deeper treats that as weak evidence, not a discovery. Where a source can prove a connection — a cryptographically signed proof, a commit's real author metadata, a certificate's registered domain — Deeper follows that instead of a lookalike username. The tool has been tuned more than once specifically to stop trusting "the page returned 200" as proof of anything.

The result is a graph, not a list: every trace has a parent it was discovered from and a reason it was kept, so you can always trace a fact backward to how Deeper found it.

## What it can find

Starting from almost any single identifier, Deeper can grow a picture across five areas:

- **Identity** — the real name, company, and role behind a username or email; company registry and legal-entity lookups; academic and professional publications.
- **Verified social presence** — accounts confirmed to belong to the same person, not just accounts that share a username. This ranges from a broad existence sweep across hundreds of platforms to focused plugins for the platforms worth digging into properly, including ones that expose cryptographically verified links between accounts.
- **Infrastructure** — the domains, subdomains, DNS records, IP ranges, and hosting footprint behind a company or website, discovered through certificate transparency, DNS enumeration, and WHOIS.
- **Code footprint** — public repositories, commit history, and the real names and emails hiding behind commit authorship, including co-authors pulled in along the way.
- **Contact surface** — emails surfaced from crawled pages and commit metadata, plus the SSH and PGP keys someone has published on their code-hosting profile.

## Getting started

```bash
git clone https://github.com/smirnoffmg/deeper.git
cd deeper
make build
./build/deeper scan <email | username | domain | company>
```

Every scan also renders its trace graph to a self-contained, interactive HTML report (`~/.deeper/reports/scan-<id>.html`) and opens it in your browser — pan, zoom, hover for details, click a trace to isolate its neighbors. Pass `--no-open` to skip the auto-open (e.g. in CI) without losing the saved report.

Architecture and performance-tuning details live in [`docs/`](docs/) — this README is deliberately just the front door.

## Responsible use

Deeper is built for legitimate OSINT research and authorized security testing. Make sure you have proper authorization before pointing it at any person, system, or organization you don't own.

## License

MIT — see [LICENSE](LICENSE).
