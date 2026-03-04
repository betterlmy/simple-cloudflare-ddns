local m, s, o

m = Map("cloudflare-ddns",
	translate("Cloudflare DDNS"),
	translate("Automatically update Cloudflare DNS records when your public IP changes."))

m.on_after_commit = function(self)
	if self.changed then
		local enabled = self.uci:get("cloudflare-ddns", "main", "enabled")
		if enabled == "1" then
			os.execute("/etc/init.d/cloudflare-ddns enable >/dev/null 2>&1")
			os.execute("/etc/init.d/cloudflare-ddns restart >/dev/null 2>&1")
		else
			os.execute("/etc/init.d/cloudflare-ddns stop >/dev/null 2>&1")
			os.execute("/etc/init.d/cloudflare-ddns disable >/dev/null 2>&1")
		end
	end
end

s = m:section(NamedSection, "main", "cloudflare-ddns")
s.addremove = false
s.anonymous = false

o = s:option(Flag, "enabled", translate("Enable"))
o.rmempty  = false
o.default  = "0"

o = s:option(Value, "api_token", translate("API Token"))
o.password = true
o.rmempty  = false

o = s:option(Value, "zone_id", translate("Zone ID"))
o.rmempty = false

o = s:option(Value, "record_name", translate("Record Name"),
	translate("e.g. home.example.com"))
o.rmempty = false

o = s:option(ListValue, "record_type", translate("Record Type"))
o:value("A",    "A (IPv4)")
o:value("AAAA", "AAAA (IPv6)")
o.default = "A"

o = s:option(Value, "check_interval", translate("Check Interval (seconds)"))
o.datatype = "uinteger"
o.default  = "300"

return m
