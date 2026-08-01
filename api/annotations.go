// Package api defines the public TDNS management API contract.
//
//	@title						TDNS Management API
//	@version					1.0
//	@description				Configure and inspect a running TDNS server.
//	@BasePath					/
//	@schemes					http https
//	@accept						json
//	@produce					json
//	@tag.name					stub-resolver
//	@tag.description			Stub resolver configuration and state.
//	@tag.name					zen-mode
//	@tag.description			Scheduled domain blocking configuration and state.
//	@tag.name					cache
//	@tag.description			DNS cache configuration and state.
//	@tag.name					blacklist
//	@tag.description			Blacklist configuration and state.
//	@tag.name					static-response
//	@tag.description			Static DNS response configuration and state.
//	@tag.name					dns-log
//	@tag.description			DNS query log reports and maintenance.
//	@tag.name					tagger
//	@tag.description			Client address labels and known hosts.
//	@tag.name					monitoring
//	@tag.description			Service monitoring endpoints.
//	@tag.name					authentication
//	@tag.description			Browser authentication session endpoints.
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Bearer token using the format "Bearer {token}".
//	@securityDefinitions.apikey	CookieAuth
//	@in							header
//	@name						Cookie
//	@description				Browser session cookie set by the browser-code exchange or password login. OpenAPI 3 defines this as the __Host-tdns-session cookie.
package api
