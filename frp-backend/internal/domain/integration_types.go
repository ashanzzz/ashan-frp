package domain

import (
	"encoding/json"
	"time"
)

// ---- ChmlFrp ----

type ChmlFrpLoginResponse struct {
	Code    int             `json:"code"`
	State   string          `json:"state,omitempty"`
	Message string          `json:"message,omitempty"`
	Msg     string          `json:"msg,omitempty"`
	Token   string          `json:"token,omitempty"`
	UserID  int             `json:"userid,omitempty"`
	Error   string          `json:"error,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type ChmlFrpDeviceAuthResp struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type ChmlFrpOAuthTokenResp struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

type ChmlFrpNode struct {
	APIToken  string `json:"apitoken,omitempty"`
	NodeToken string `json:"nodetoken,omitempty"`
	ID        int    `json:"id,omitempty"`
	IP        string `json:"ip,omitempty"`
	RealIP    string `json:"realIp,omitempty"`
	Name      string `json:"name,omitempty"`
	Area      string `json:"area,omitempty"`
	China     string `json:"china,omitempty"`
	Fangyu    string `json:"fangyu,omitempty"`
	HTTPPort  int    `json:"http_port,omitempty"`
	HTTPSPort int    `json:"https_port,omitempty"`
	NodeGroup string `json:"nodegroup,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Port      int    `json:"port,omitempty"`
	RPort     string `json:"rport,omitempty"`
	UDP       string `json:"udp,omitempty"`
	Wed       string `json:"wed,omitempty"`
}

type ChmlFrpTunnel struct {
	ID            string      `json:"id,omitempty"`
	TunnelID      json.Number `json:"tunnelID,omitempty"`
	Name          string      `json:"name,omitempty"`
	TunnelName    string      `json:"tunnelName,omitempty"`
	Node          string      `json:"node,omitempty"`
	Type          string      `json:"type,omitempty"`
	PortType      string      `json:"portType,omitempty"`
	AP            string      `json:"ap,omitempty"`
	IP            string      `json:"ip,omitempty"`
	LocalIP       string      `json:"localip,omitempty"`
	LocalIP_V2    string      `json:"localIP,omitempty"`
	LocalPort     int         `json:"local_port,omitempty"`
	LocalPort_V2  int         `json:"localPort,omitempty"`
	RemotePort    int         `json:"remote_port,omitempty"`
	RemotePort_V2 int         `json:"remotePort,omitempty"`
	NPort         string      `json:"nport,omitempty"`
	DPort         string      `json:"dport,omitempty"`
	Dorp          string      `json:"dorp,omitempty"`
	Domain        string      `json:"domain,omitempty"`
	BandDomain    string      `json:"band_domain,omitempty"`
	BandDomain_V2 string      `json:"bandDomain,omitempty"`
	State         string      `json:"state,omitempty"`
	TunnelState   string      `json:"tunnelState,omitempty"`
	NodeState     string      `json:"nodestate,omitempty"`
	Encryption    string      `json:"encryption,omitempty"`
	Compression   string      `json:"compression,omitempty"`
	ClientVersion string      `json:"client_version,omitempty"`
	Uptime        string      `json:"uptime,omitempty"`
	UserID        int         `json:"userid,omitempty"`
	UserID_V2     int         `json:"userID,omitempty"`
	CurConns      int         `json:"cur_conns,omitempty"`
}

func (t *ChmlFrpTunnel) GetID() string {
	if t.ID != "" {
		return t.ID
	}
	return t.TunnelID.String()
}

func (t *ChmlFrpTunnel) GetName() string {
	if t.Name != "" {
		return t.Name
	}
	return t.TunnelName
}

func (t *ChmlFrpTunnel) GetType() string {
	if t.Type != "" {
		return t.Type
	}
	return t.PortType
}

func (t *ChmlFrpTunnel) GetLocalIP() string {
	if t.LocalIP != "" {
		return t.LocalIP
	}
	if t.LocalIP_V2 != "" {
		return t.LocalIP_V2
	}
	return "127.0.0.1"
}

func (t *ChmlFrpTunnel) GetLocalPort() int {
	if t.LocalPort > 0 {
		return t.LocalPort
	}
	return t.LocalPort_V2
}

func (t *ChmlFrpTunnel) GetRemotePort() int {
	if t.RemotePort > 0 {
		return t.RemotePort
	}
	return t.RemotePort_V2
}

func (t *ChmlFrpTunnel) GetDomain() string {
	if t.Domain != "" {
		return t.Domain
	}
	return t.BandDomain_V2
}

type ChmlFrpCreateTunnelReq struct {
	TunnelName  string `json:"tunnel_name,omitempty"`
	Node        string `json:"node,omitempty"`
	PortType    string `json:"port_type,omitempty"`
	ExtraParams string `json:"extra_params,omitempty"`
	LocalIP     string `json:"local_ip,omitempty"`
	LocalPort   int    `json:"local_port,omitempty"`
	RemotePort  int    `json:"remote_port,omitempty"`
	BandDomain  string `json:"band_domain,omitempty"`
	Encryption  bool   `json:"encryption,omitempty"`
	Compression bool   `json:"compression,omitempty"`
}

type ChmlFrpConfigResponse struct {
	Status  int    `json:"status,omitempty"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Msg     string `json:"msg,omitempty"`
	Code    int    `json:"code,omitempty"`
	State   string `json:"state,omitempty"`
	Data    string `json:"data,omitempty"`
}

type ChmlFrpV2CreateTunnelBody struct {
	Token       string `json:"token"`
	TunnelName  string `json:"tunnelname"`
	Node        string `json:"node"`
	LocalIP     string `json:"localip"`
	PortType    string `json:"porttype"`
	LocalPort   int    `json:"localport"`
	RemotePort  int    `json:"remoteport,omitempty"`
	BandDomain  string `json:"banddomain,omitempty"`
	Encryption  bool   `json:"encryption"`
	Compression bool   `json:"compression"`
	ExtraParams string `json:"extraparams,omitempty"`
}

type ChmlFrpNodeInfoResp struct {
	Code int `json:"code"`
	Data struct {
		State  string `json:"state"`
		RealIP string `json:"realIp"`
		IP     string `json:"ip"`
	} `json:"data"`
}

// ---- Cloudflare ----

type CFErrorSource struct {
	Pointer string `json:"pointer,omitempty"`
}

type CFAPIError struct {
	Code             int            `json:"code"`
	Message          string         `json:"message"`
	DocumentationURL string         `json:"documentation_url,omitempty"`
	Source           *CFErrorSource `json:"source,omitempty"`
}

type CFResultInfo struct {
	Count      int `json:"count,omitempty"`
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	TotalCount int `json:"total_count,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

type CFDNSRecord struct {
	ID                string         `json:"id,omitempty"`
	Name              string         `json:"name,omitempty"`
	Type              string         `json:"type,omitempty"`
	Content           string         `json:"content,omitempty"`
	PrivateRouting    bool           `json:"private_routing,omitempty"`
	Proxiable         bool           `json:"proxiable,omitempty"`
	Proxied           bool           `json:"proxied,omitempty"`
	Settings          map[string]any `json:"settings,omitempty"`
	Meta              map[string]any `json:"meta,omitempty"`
	Comment           string         `json:"comment,omitempty"`
	Tags              []string       `json:"tags,omitempty"`
	TTL               int            `json:"ttl,omitempty"`
	Priority          *int           `json:"priority,omitempty"`
	Data              map[string]any `json:"data,omitempty"`
	CreatedOn         time.Time      `json:"created_on,omitempty"`
	ModifiedOn        time.Time      `json:"modified_on,omitempty"`
	CommentModifiedOn *time.Time     `json:"comment_modified_on,omitempty"`
	TagsModifiedOn    *time.Time     `json:"tags_modified_on,omitempty"`
}

type CFZone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
	Paused bool   `json:"paused,omitempty"`
	Type   string `json:"type,omitempty"`
}

type CFZoneListResponse struct {
	Success    bool          `json:"success"`
	Errors     []CFAPIError  `json:"errors,omitempty"`
	Messages   []CFAPIError  `json:"messages,omitempty"`
	Result     []CFZone      `json:"result,omitempty"`
	ResultInfo *CFResultInfo `json:"result_info,omitempty"`
}

type CFListResponse struct {
	Success    bool          `json:"success"`
	Errors     []CFAPIError  `json:"errors,omitempty"`
	Messages   []CFAPIError  `json:"messages,omitempty"`
	Result     []CFDNSRecord `json:"result,omitempty"`
	ResultInfo *CFResultInfo `json:"result_info,omitempty"`
}

type CFResponse struct {
	Success  bool            `json:"success"`
	Errors   []CFAPIError    `json:"errors,omitempty"`
	Messages []CFAPIError    `json:"messages,omitempty"`
	Result   json.RawMessage `json:"result,omitempty"`
}

type CFCreateRecordReq struct {
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Content  string         `json:"content,omitempty"`
	Proxied  bool           `json:"proxied,omitempty"`
	TTL      int            `json:"ttl,omitempty"`
	Comment  string         `json:"comment,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Priority *int           `json:"priority,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

type CAARecordData struct {
	Flags int    `json:"flags"`
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

type DNSRecordInput struct {
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Content  string         `json:"content,omitempty"`
	TTL      int            `json:"ttl"`
	Proxied  *bool          `json:"proxied,omitempty"`
	Priority *int           `json:"priority,omitempty"`
	CAA      *CAARecordData `json:"caa,omitempty"`
}

// ---- OnePanel ----

type OnePanelCreateWebsiteReq struct {
	PrimaryDomain  string   `json:"primaryDomain"`
	Alias          string   `json:"alias,omitempty"`
	Remark         string   `json:"remark,omitempty"`
	Type           string   `json:"type"`
	Proxy          string   `json:"proxy,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	WebsiteGroupID int      `json:"webSiteGroupID,omitempty"`
	AppType        string   `json:"appType,omitempty"`
	AppInstallID   int      `json:"appInstallId,omitempty"`
	OtherDomains   string   `json:"otherDomains,omitempty"`
	ProxyType      string   `json:"proxyType,omitempty"`
	FtpUser        string   `json:"ftpUser,omitempty"`
	FtpPassword    string   `json:"ftpPassword,omitempty"`
	IPV6           bool     `json:"IPV6,omitempty"`
}

type OnePanelResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message,omitempty"`
	Msg     string          `json:"msg,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type OnePanelWebsite struct {
	ID            int       `json:"id"`
	CreatedAt     time.Time `json:"createdAt,omitempty"`
	Protocol      string    `json:"protocol,omitempty"`
	PrimaryDomain string    `json:"primaryDomain,omitempty"`
	Type          string    `json:"type,omitempty"`
	Alias         string    `json:"alias,omitempty"`
	Remark        string    `json:"remark,omitempty"`
	Status        string    `json:"status,omitempty"`
	ExpireDate    time.Time `json:"expireDate,omitempty"`
	SitePath      string    `json:"sitePath,omitempty"`
	AppName       string    `json:"appName,omitempty"`
	RuntimeName   string    `json:"runtimeName,omitempty"`
	SSLExpireDate time.Time `json:"sslExpireDate,omitempty"`
	SSLStatus     string    `json:"sslStatus,omitempty"`
	AppInstallID  int       `json:"appInstallId,omitempty"`
	ChildSites    []string  `json:"childSites,omitempty"`
	ParentSite    string    `json:"parentSite,omitempty"`
	RuntimeType   string    `json:"runtimeType,omitempty"`
	Favorite      bool      `json:"favorite,omitempty"`
	IPV6          bool      `json:"IPV6,omitempty"`
}
