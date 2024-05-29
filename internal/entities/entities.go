package entities

import (
	"log"
	"regexp"
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
	Twitter   TraceType = "twitter"
	Github    TraceType = "github"
	Linkedin  TraceType = "linkedin"
	Instagram TraceType = "instagram"
	Facebook  TraceType = "facebook"
	TikTok    TraceType = "tiktok"
	Reddit    TraceType = "reddit"
	YouTube   TraceType = "youtube"
	Pinterest TraceType = "pinterest"
	Snapchat  TraceType = "snapchat"
	Tumblr    TraceType = "tumblr"
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
	// Network traces
	Subdomain TraceType = "subdomain"
	ASN       TraceType = "asn"
	Netblock  TraceType = "netblock"
	Host      TraceType = "host"
	IPRange   TraceType = "ip_range"
)

type Trace struct {
	Value string
	Type  TraceType
}

func (t Trace) String() string {
	return t.Value + " (" + string(t.Type) + ")"
}

func NewTrace(value string) Trace {
	return Trace{
		Value: value,
		Type:  guessTraceType(value),
	}
}

// Add functions for new trace types
func isEmail(value string) bool {
	emailRegex := "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
	return regexp.MustCompile(emailRegex).MatchString(value)
}

func isPhone(value string) bool {
	phoneRegex := `^(\+?(\d{1,3}))?[-. ]?(\(?\d{3}\)?[-. ]?)?(\d{3})[-. ]?(\d{4})$`
	return regexp.MustCompile(phoneRegex).MatchString(value)
}

func isAddress(value string) bool {
	addressRegex := `^\d+\s[A-z]+\s[A-z]+`
	return regexp.MustCompile(addressRegex).MatchString(value)
}

func isIpAddr(value string) bool {
	ipAddrRegex := `^(\d{1,3}\.){3}\d{1,3}$`
	return regexp.MustCompile(ipAddrRegex).MatchString(value)
}

func isDomain(value string) bool {
	domainRegex := `^([a-zA-Z0-9]+\.){1,}([a-zA-Z]{2,})$`
	return regexp.MustCompile(domainRegex).MatchString(value)
}

func isUrl(value string) bool {
	urlRegex := `^https?://[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	return regexp.MustCompile(urlRegex).MatchString(value)
}

func isTwitterHandle(value string) bool {
	twitterRegex := `^@[a-zA-Z0-9_]{1,15}$`
	return regexp.MustCompile(twitterRegex).MatchString(value)
}

func isGithubUsername(value string) bool {
	githubRegex := `^[a-zA-Z0-9-]{1,39}$`
	return regexp.MustCompile(githubRegex).MatchString(value)
}

func isLinkedinProfile(value string) bool {
	linkedinRegex := `^https?://(www\.)?linkedin\.com/in/[a-zA-Z0-9-_]+/?$`
	return regexp.MustCompile(linkedinRegex).MatchString(value)
}

func isInstagramHandle(value string) bool {
	instagramRegex := `^@[a-zA-Z0-9._]{1,30}$`
	return regexp.MustCompile(instagramRegex).MatchString(value)
}

func isFacebookProfile(value string) bool {
	facebookRegex := `^https?://(www\.)?facebook\.com/[a-zA-Z0-9._-]+/?$`
	return regexp.MustCompile(facebookRegex).MatchString(value)
}

func isTikTokHandle(value string) bool {
	tiktokRegex := `^@[a-zA-Z0-9._]{1,30}$`
	return regexp.MustCompile(tiktokRegex).MatchString(value)
}

func isRedditUsername(value string) bool {
	redditRegex := `^u/[a-zA-Z0-9-_]{3,20}$`
	return regexp.MustCompile(redditRegex).MatchString(value)
}

func isYouTubeChannel(value string) bool {
	youtubeRegex := `^https?://(www\.)?youtube\.com/channel/[a-zA-Z0-9_-]+/?$`
	return regexp.MustCompile(youtubeRegex).MatchString(value)
}

func isPinterestProfile(value string) bool {
	pinterestRegex := `^https?://(www\.)?pinterest\.com/[a-zA-Z0-9_]+/?$`
	return regexp.MustCompile(pinterestRegex).MatchString(value)
}

func isSnapchatHandle(value string) bool {
	snapchatRegex := `^@[a-zA-Z0-9._-]{1,15}$`
	return regexp.MustCompile(snapchatRegex).MatchString(value)
}

func isTumblrBlog(value string) bool {
	tumblrRegex := `^[a-zA-Z0-9-]+\.tumblr\.com$`
	return regexp.MustCompile(tumblrRegex).MatchString(value)
}

func isMacAddr(value string) bool {
	macAddrRegex := `^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`
	return regexp.MustCompile(macAddrRegex).MatchString(value)
}

func isBitcoinAddress(value string) bool {
	bitcoinRegex := `^1[a-km-zA-HJ-NP-Z1-9]{25,34}$`
	return regexp.MustCompile(bitcoinRegex).MatchString(value)
}

func guessTraceType(value string) TraceType {
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
		log.Printf("Unknown trace type for value: %s", value)
		return Username
	}
}
