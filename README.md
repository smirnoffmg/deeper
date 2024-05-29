# Deeper

```
██████  ███████ ███████ ██████  ███████ ██████  
██   ██ ██      ██      ██   ██ ██      ██   ██ 
██   ██ █████   █████   ██████  █████   ██████  
██   ██ ██      ██      ██      ██      ██   ██ 
██████  ███████ ███████ ██      ███████ ██   ██ 
```

Deeper is an easy-to-use OSINT tool that leverages plugins to expand its capabilities.

## Plugin Example

Here is an example of how to create a new plugin for Deeper:

```go
package new_plugin

import (
    "github.com/smirnoffmg/deeper/internal/entities"
    "github.com/smirnoffmg/deeper/internal/state"
)

//Use the appropriate TraceType
const InputTraceType = entities.Username

func init() {
    p := NewPlugin()
    p.Register()
}

type NewPlugin struct{}

func NewPlugin() *NewPlugin {
    return &NewPlugin{}
}

func (g *NewPlugin) Register() error {
    state.RegisterPlugin(InputTraceType, g)
    return nil
}

func (g *NewPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
    if trace.Type != InputTraceType {
        return nil, nil
    }

    // Your logic for processing the trace
    var newTraces []entities.Trace
    // Example: Add a new trace based on the input trace
    newTraces = append(newTraces, entities.Trace{
        Value: "example_trace_value",
        Type:  InputTraceType,
    })
    return newTraces, nil
}

func (g NewPlugin) String() string {
    return "NewPlugin"
}
```

## How to Contribute

We welcome contributions from the community to help improve Deeper!

1. __Fork the Repository__:
   - Click the "Fork" button at the top right of the repository page.

2. __Create a New Branch__:
   - Clone your forked repository locally: `git clone https://github.com/your-username/deeper.git`
   - Create a new branch for your feature or bug fix: `git checkout -b feature-branch`

3. __Add Your Plugin Code__:
   - Add your new plugin code following the provided example.

4. __Commit Your Changes__:
   - Commit your changes with a descriptive message: `git commit -am 'Add new plugin'`

5. __Push to Your Branch__:
   - Push your changes to GitHub: `git push origin feature-branch`

6. __Create a New Pull Request__:
   - Go to the original repository and click the "New Pull Request" button.
   - Choose your branch and submit the pull request for review.

Thank you for your contributions!

## To-do list

- [ ] __BitcoinAddress -> Blockchain Analysis__
- [ ] __BitcoinAddress -> Transactions Analysis__
- [ ] __BitcoinAddress -> Transactions, Associated Wallets__
- [ ] __BitcoinAddress -> Wallet Balance__
- [ ] __Company -> Company Details, Employee Emails__
- [ ] __DnsRecord -> Subdomain, IpAddr__
- [ ] __Domain -> AAAA Records__
- [ ] __Domain -> CNAME Records__
- [ ] __Domain -> MX Records__
- [ ] __Domain -> NS Records__
- [ ] __Domain -> PTR Records__
- [ ] __Domain -> Redirect Chains__
- [ ] __Domain -> Reverse DNS__
- [ ] __Domain -> Sitelinks__
- [ ] __Domain -> SOA Records__
- [ ] __Domain -> SPF Records__
- [ ] __Domain -> SSL Certificates__
- [x] __Domain -> Subdomain Enumeration__
- [ ] __Domain -> Subdomain, IpAddr__
- [ ] __Domain -> TXT Records__
- [ ] __Domain -> Web Technologies__
- [ ] __Domain -> WHOIS History__
- [ ] __Domain -> WHOIS Information__
- [ ] __Domain -> Zone Transfers__
- [ ] __Email -> Academic Papers__
- [ ] __Email -> Associated Domains__
- [ ] __Email -> Data Aggregators__
- [ ] __Email -> Data Breaches__
- [ ] __Email -> Data Mining__
- [ ] __Email -> Email Verification__
- [ ] __Email -> Facebook Profiles__
- [ ] __Email -> Google Scholar Profiles__
- [ ] __Email -> Gravatar__
- [ ] __Email -> LinkedIn Connections__
- [ ] __Email -> LinkedIn Profiles__
- [ ] __Email -> Public Record Search__
- [ ] __Email -> Reverse Email Search__
- [ ] __Email -> Snapchat Profiles__
- [ ] __Email -> Social Media Profiles__
- [ ] __Email -> Twitter Profiles__
- [ ] __ExifData -> Geolocation, Device Information__
- [ ] __Geolocation -> Nearby Facilities__
- [ ] __IpAddr -> Abuse Reports__
- [ ] __IpAddr -> ASN Information__
- [ ] __IpAddr -> DNS Lookup__
- [ ] __IpAddr -> Domain Names__
- [ ] __IpAddr -> Geolocation, ASN__
- [ ] __IpAddr -> Hosting Provider__
- [ ] __IpAddr -> IP Geolocation__
- [ ] __IpAddr -> IP History__
- [ ] __IpAddr -> Network Info__
- [ ] __IpAddr -> Port Scan__
- [ ] __IpAddr -> Service Detection__
- [ ] __IpAddr -> Threat Intelligence__
- [ ] __IpAddr -> Traffic Analysis__
- [ ] __IpAddr -> WHOIS Info__
- [ ] __MacAddr -> Device Information__
- [ ] __Name -> Academic Publications__
- [ ] __Name -> Address History__
- [ ] __Name -> Business Records__
- [ ] __Name -> Facebook Profiles__
- [ ] __Name -> Google Scholar Profiles__
- [ ] __Name -> Instagram Profiles__
- [ ] __Name -> LinkedIn Profiles__
- [ ] __Name -> Public Records__
- [ ] __Name -> Relatives__
- [ ] __Name -> Social Media Profiles, Public Records__
- [ ] __PayPalAccount -> Transactions__
- [ ] __Phone -> Business Listings__
- [ ] __Phone -> Carrier Info__
- [ ] __Phone -> Data Breaches__
- [ ] __Phone -> Location__
- [ ] __Phone -> Nuisance Reports__
- [ ] __Phone -> Public Directories__
- [ ] __Phone -> Public Profiles__
- [ ] __Phone -> Skype Profiles__
- [ ] __Phone -> Social Media Profiles__
- [ ] __Phone -> Telegram Profiles__
- [ ] __Phone -> Verification Services__
- [ ] __Phone -> WhatsApp Profiles__
- [ ] __Phone -> White Pages__
- [ ] __Subdomain -> Associated Domains__
- [ ] __Subdomain -> IP Addresses__
- [ ] __Subdomain -> IpAddr, DnsRecord__
- [ ] __Subdomain -> Reverse DNS__
- [ ] __Subdomain -> Security Reports__
- [ ] __Subdomain -> SOA Records__
- [ ] __Subdomain -> SPF Records__
- [ ] __Url -> Archived Pages__
- [ ] __Url -> Domain, Subdomain__
- [x] __Username -> Code Repositories__
- [ ] __Username -> Data Breaches__
- [ ] __Username -> DeviantArt Profiles__
- [ ] __Username -> Email Addresses__
- [ ] __Username -> Facebook Profiles__
- [ ] __Username -> GitHub Repos__
- [ ] __Username -> Instagram Profiles__
- [ ] __Username -> LinkedIn Profiles__
- [ ] __Username -> Medium Profiles__
- [ ] __Username -> Pastebin Dumps__
- [ ] __Username -> Pinterest Profiles__
- [ ] __Username -> Quora Profiles__
- [ ] __Username -> Reddit Profiles__
- [ ] __Username -> Social Media Profiles__
- [ ] __Username -> Social Mention__
- [ ] __Username -> Steam Profiles__
- [ ] __Username -> Tumblr Profiles__
- [ ] __Username -> Twitter Profiles__
