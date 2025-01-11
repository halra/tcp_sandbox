package domain

import "encoding/xml"

// KeepAliveXML represents the structure of the keep-alive XML file.
type KeepAliveXML struct {
	XMLName    xml.Name `xml:"keepAlive"`
	TenantName string   `xml:"tenantName"`
	SendTime   string   `xml:"sendTime"`
}
