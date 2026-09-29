package entities

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type TraceType string

const (
	Email    TraceType = "email"
	Phone    TraceType = "phone"
	Address  TraceType = "address"
	IpAddr   TraceType = "ip_addr"
	Domain   TraceType = "domain"
	Url      TraceType = "url"
	Username TraceType = "username"
	Name     TraceType = "name"
	Company  TraceType = "company"
	// Personal data
	Alias                   TraceType = "alias"
	DateOfBirth             TraceType = "date_of_birth"
	Gender                  TraceType = "gender"
	Nationality             TraceType = "nationality"
	MacAddr                 TraceType = "mac_addr"
	SSHKey                  TraceType = "ssh_key"
	PGPKey                  TraceType = "pgp_key"
	BitcoinAddress          TraceType = "bitcoin_address"
	PayPalAccount           TraceType = "paypal_account"
	MedicalRecordNumber     TraceType = "medical_record_number"
	InsurancePolicy         TraceType = "insurance_policy"
	ExifData                TraceType = "exif_data"
	FileTimestamp           TraceType = "file_timestamp"
	Geolocation             TraceType = "geolocation"
	ForumRegistrations      TraceType = "forum_registrations"
	CommentsAndPosts        TraceType = "comments_and_posts"
	NewsMentions            TraceType = "news_mentions"
	CourtRecords            TraceType = "court_records"
	Patents                 TraceType = "patents"
	Publications            TraceType = "publications"
	EducationalInstitution  TraceType = "educational_institution"
	Workplace               TraceType = "workplace"
	Certificates            TraceType = "certificates"
	ConferenceParticipation TraceType = "conference_participation"
	// Social media traces
	SocialGeneric TraceType = "social_generic"
	Twitter       TraceType = "twitter"
	Github        TraceType = "github"
	Linkedin      TraceType = "linkedin"
	Instagram     TraceType = "instagram"
	Facebook      TraceType = "facebook"
	TikTok        TraceType = "tiktok"
	Reddit        TraceType = "reddit"
	YouTube       TraceType = "youtube"
	Pinterest     TraceType = "pinterest"
	Snapchat      TraceType = "snapchat"
	Tumblr        TraceType = "tumblr"
	// Technical traces
	Repository TraceType = "repository"
	// DNS traces
	DnsRecordA     TraceType = "dns_record_a"
	DnsRecordAAAA  TraceType = "dns_record_aaaa"
	DnsRecordMX    TraceType = "dns_record_mx"
	DnsRecordNS    TraceType = "dns_record_ns"
	DnsRecordTXT   TraceType = "dns_record_txt"
	DnsRecordCNAME TraceType = "dns_record_cname"
	DnsRecordSOA   TraceType = "dns_record_soa"
	DnsRecordPTR   TraceType = "dns_record_ptr"
	DnsRecordSRV   TraceType = "dns_record_srv"
	DnsRecordCAA   TraceType = "dns_record_caa"
	Whois          TraceType = "whois"
	// Network traces
	Subdomain TraceType = "subdomain"
	ASN       TraceType = "asn"
	Netblock  TraceType = "netblock"
	Host      TraceType = "host"
	IPRange   TraceType = "ip_range"
	// Active-recon and fingerprint traces
	Technology TraceType = "technology"
	Port       TraceType = "port"
	Service    TraceType = "service"
)

type Trace struct {
	Value string
	Type  TraceType
}

// Discovery records a parent trace, the plugin that derived a child, and the child trace.
type Discovery struct {
	Parent     Trace
	PluginName string
	Child      Trace
}

func (t Trace) String() string {
	return t.Value + " (" + string(t.Type) + ")"
}

func NewTrace(value string) Trace {
	traceType := guessTraceType(value)
	if traceType == Domain {
		return Trace{Value: canonicalHostname(value), Type: traceType}
	}
	return Trace{Value: value, Type: traceType}
}

var (
	emailRegex     = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	phoneRegex     = regexp.MustCompile(`^(\+?(\d{1,3}))?[-. ]?(\(?\d{3}\)?[-. ]?)?(\d{3})[-. ]?(\d{4})$`)
	addressRegex   = regexp.MustCompile(`^\d+\s[A-z]+\s[A-z]+`)
	ipAddrRegex    = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	urlRegex       = regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	twitterRegex   = regexp.MustCompile(`^@[a-zA-Z0-9_]{1,15}$`)
	linkedinRegex  = regexp.MustCompile(`^https?://(www\.)?linkedin\.com/in/[a-zA-Z0-9-_]+/?$`)
	instagramRegex = regexp.MustCompile(`^@[a-zA-Z0-9._]{1,30}$`)
	facebookRegex  = regexp.MustCompile(`^https?://(www\.)?facebook\.com/[a-zA-Z0-9._-]+/?$`)
	tiktokRegex    = regexp.MustCompile(`^@[a-zA-Z0-9._]{1,30}$`)
	redditRegex    = regexp.MustCompile(`^u/[a-zA-Z0-9-_]{3,20}$`)
	youtubeRegex   = regexp.MustCompile(`^https?://(www\.)?youtube\.com/channel/[a-zA-Z0-9_-]+/?$`)
	pinterestRegex = regexp.MustCompile(`^https?://(www\.)?pinterest\.com/[a-zA-Z0-9_]+/?$`)
	snapchatRegex  = regexp.MustCompile(`^@[a-zA-Z0-9._-]{1,15}$`)
	tumblrRegex    = regexp.MustCompile(`^[a-zA-Z0-9-]+\.tumblr\.com$`)
	macAddrRegex   = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)
	bitcoinRegex   = regexp.MustCompile(`^1[a-km-zA-HJ-NP-Z1-9]{25,34}$`)
)

// Add functions for new trace types
func isEmail(value string) bool {
	return emailRegex.MatchString(value)
}

func isPhone(value string) bool {
	return phoneRegex.MatchString(value)
}

func isAddress(value string) bool {
	return addressRegex.MatchString(value)
}

func isIpAddr(value string) bool {
	return ipAddrRegex.MatchString(value)
}

func isDomain(value string) bool {
	return hostnameError(value) == nil
}

func isUrl(value string) bool {
	return urlRegex.MatchString(value)
}

func isTwitterHandle(value string) bool {
	return twitterRegex.MatchString(value)
}

func isLinkedinProfile(value string) bool {
	return linkedinRegex.MatchString(value)
}

func isInstagramHandle(value string) bool {
	return instagramRegex.MatchString(value)
}

func isFacebookProfile(value string) bool {
	return facebookRegex.MatchString(value)
}

func isTikTokHandle(value string) bool {
	return tiktokRegex.MatchString(value)
}

func isRedditUsername(value string) bool {
	return redditRegex.MatchString(value)
}

func isYouTubeChannel(value string) bool {
	return youtubeRegex.MatchString(value)
}

func isPinterestProfile(value string) bool {
	return pinterestRegex.MatchString(value)
}

func isSnapchatHandle(value string) bool {
	return snapchatRegex.MatchString(value)
}

func isTumblrBlog(value string) bool {
	return tumblrRegex.MatchString(value)
}

func isMacAddr(value string) bool {
	return macAddrRegex.MatchString(value)
}

func isBitcoinAddress(value string) bool {
	return bitcoinRegex.MatchString(value)
}

// IsEmail reports whether value is shaped like a real email address.
// Exported for plugins outside this package that need to validate a value
// before trusting it as an Email-typed trace (see contact_crawler, github_*).
func IsEmail(value string) bool {
	return isEmail(value)
}

// IsFacebookProfile reports whether value is shaped like a real Facebook
// profile URL (as opposed to a share widget, tracking link, or bare host).
func IsFacebookProfile(value string) bool {
	return isFacebookProfile(value)
}

// IsIpAddr reports whether value is a clean, well-formed IPv4 address.
func IsIpAddr(value string) bool {
	return isIpAddr(value)
}

// reservedEmailDomains are IANA-reserved for documentation (RFC 2606) and
// can never be a real contact address.
var reservedEmailDomains = map[string]bool{
	"example.com": true,
	"example.org": true,
	"example.net": true,
}

// IsRealEmail reports whether value is both shaped like an email and not at
// a reserved documentation-only domain -- e.g. "you@example.com", git's
// unconfigured-user.email default, is syntactically valid but never a real
// contact address.
func IsRealEmail(value string) bool {
	if !isEmail(value) {
		return false
	}
	domain := strings.ToLower(value[strings.LastIndex(value, "@")+1:])
	return !reservedEmailDomains[domain]
}

// logRollback is the only place a Username rollback is logged, so each
// rollback produces exactly one log line whichever path it takes.
func logRollback(value, reason string) {
	log.Info().
		Str("value", truncateForLog(value)).
		Int("value_len", len(value)).
		Str("reason", reason).
		Msg("could not guess trace type, rolling back to username")
}

func guessTraceType(value string) TraceType {
	// Must precede every matcher: each one scans the whole input, so this is
	// what bounds classification cost for untrusted values.
	if len(value) > maxClassifiableLen {
		logRollback(value, errValueTooLong.Error())
		return Username
	}

	switch {
	case isEmail(value):
		return Email
	case isPhone(value):
		return Phone
	case isIpAddr(value):
		return IpAddr
	case isDomain(value):
		return Domain
	case isUrl(value):
		return Url
	case isAddress(value):
		return Address
	case isTwitterHandle(value):
		return Twitter
	case isLinkedinProfile(value):
		return Linkedin
	case isInstagramHandle(value):
		return Instagram
	case isFacebookProfile(value):
		return Facebook
	case isTikTokHandle(value):
		return TikTok
	case isRedditUsername(value):
		return Reddit
	case isYouTubeChannel(value):
		return YouTube
	case isPinterestProfile(value):
		return Pinterest
	case isSnapchatHandle(value):
		return Snapchat
	case isTumblrBlog(value):
		return Tumblr
	case isMacAddr(value):
		return MacAddr
	case isBitcoinAddress(value):
		return BitcoinAddress
	default:
		logRollback(value, hostnameError(value).Error())
		return Username
	}
}
