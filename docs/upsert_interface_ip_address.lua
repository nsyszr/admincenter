-----LUA-----
function exists_interface_ip_address(ifname, ip_address, ip_netmask)
    for i = 1, tonumber(cli("interfaces." .. ifname .. ".ip_address.size")) do
        local addr = cli("interfaces." .. ifname .. ".ip_address[" .. i .. "].ip_address")
        local mask = cli("interfaces." .. ifname .. ".ip_address[" .. i .. "].netmask")
        if ip_address == addr and tonumber(ip_netmask) == tonumber(mask) then
            return i
        end
    end
    return 0
end

function upsert_interface_ip_address(ifname, is_active, ip_address, ip_netmask, descr)
    local i = exists_interface_ip_address(ifname, ip_address, ip_netmask)
    if i > 0 then
        cli("interfaces." .. ifname .. ".ip_address[" .. i .. "].ip_active=" .. is_active)
        cli("interfaces." .. ifname .. ".ip_address[" .. i .. "].ip_address=" .. ip_address)
        cli("interfaces." .. ifname .. ".ip_address[" .. i .. "].netmask=" .. ip_netmask)
        cli("interfaces." .. ifname .. ".ip_address[" .. i .. "].ip_description=" .. descr)
    else
        cli("interfaces." .. ifname .. ".ip_address.add")
        cli("interfaces." .. ifname .. ".ip_address[last].ip_active=" .. is_active)
        cli("interfaces." .. ifname .. ".ip_address[last].ip_address=" .. ip_address)
        cli("interfaces." .. ifname .. ".ip_address[last].netmask=" .. ip_netmask)
        cli("interfaces." .. ifname .. ".ip_address[last].ip_description=" .. descr)
    end
end

upsert_interface_ip_address("net2", 1, "192.168.2.1", 24, "Test1")
upsert_interface_ip_address("net2", 1, "192.168.3.1", 24, "Test2")
-----LUA-----