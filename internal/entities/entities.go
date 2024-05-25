package entities

import "regexp"

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

	// social
	Twitter   TraceType = "twitter"
	Github    TraceType = "github"
	Linkedin  TraceType = "linkedin"
	Instagram TraceType = "instagram"
	Facebook  TraceType = "facebook"

	// tech
	Repository TraceType = "repository"

	// dns
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

	// network
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

func guessTraceType(value string) TraceType {
	if isEmail(value) {
		return Email
	}

	if isPhone(value) {
		return Phone
	}

	if isIpAddr(value) {
		return IpAddr
	}

	if isDomain(value) {
		return Domain
	}

	if isUrl(value) {
		return Url
	}

	if isAddress(value) {
		return Address
	}

	return Username
}
