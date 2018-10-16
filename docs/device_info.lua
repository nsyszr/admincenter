-----LUA-----
function yaml_indent(level)
    local s = ""
    for i = 1, level do s = s .. "  " end
    return s
end

function yaml_section(level, section)
    return yaml_indent(level) .. section .. ":\n"
end

function yaml_string(level, key, value)
    local s = yaml_indent(level) .. key .. ": "
    if not (value == nil or value == "") then
        s = s .. "\"" .. (value:gsub("^%s*(.-)%s*$", "%1")) .. "\""
    end
    return s .. "\n"
end

function yaml_number(level, key, value)
    local s = yaml_indent(level) .. key .. ": "
    local v = tonumber(value)
    if not (v == nil) then
        s = s .. v
    end
    return s .. "\n"
end

function device_info()
    local yml = yaml_section(0, "device_info")
    for i = 1, tonumber(cli("status.device_info.slot.size")) do
        local board_type = cli("status.device_info.slot[" .. i .. "].board_type")
        local hardware_features = cli("status.device_info.slot[" .. i .. "].hardware_features")
        local hardware_version = cli("status.device_info.slot[" .. i .. "].hardware_version")
        local firmware_version = cli("status.device_info.slot[" .. i .. "].firmware_version")
        local serial_number = cli("status.device_info.slot[" .. i .. "].serial_number")
        yml = yml .. "- " .. yaml_string(0, "board_type", board_type)
        yml = yml .. yaml_string(1, "hardware_features", hardware_features)
        yml = yml .. yaml_string(1, "hardware_version", hardware_version)
        yml = yml .. yaml_string(1, "firmware_version", firmware_version)
        yml = yml .. yaml_string(1, "serial_number", serial_number)
    end
    return yml
end

function sysdetail()
    local yml = yaml_section(0, "sysdetail")

    -- system
    local location = cli("status.sysdetail.system.location")
    local date = cli("status.sysdetail.system.date")
    local uptime = cli("status.sysdetail.system.uptime")
    local load = cli("status.sysdetail.system.load")
    local ram = cli("status.sysdetail.system.ram")
    local hash = cli("status.sysdetail.system.hash")
    local mac = cli("status.sysdetail.system.mac")
    yml = yml .. yaml_section(1, "system")
    yml = yml .. yaml_string(2, "location", location)
    yml = yml .. yaml_string(2, "date", date)
    yml = yml .. yaml_string(2, "uptime", uptime)
    yml = yml .. yaml_string(2, "load", load)
    yml = yml .. yaml_string(2, "ram", ram)
    yml = yml .. yaml_string(2, "hash", hash)
    yml = yml .. yaml_string(2, "mac", mac)

    -- ip_addresses
    -- status.sysdetail.ip_addresses.interface
    yml = yml .. yaml_section(1, "ip_addresses")
    yml = yml .. yaml_section(2, "interfaces")
    for i = 1, tonumber(cli("status.sysdetail.ip_addresses.interface.size")) do
        local name = cli("status.sysdetail.ip_addresses.interface[" .. i .. "].name")
        local ip_address = cli("status.sysdetail.ip_addresses.interface[" .. i .. "].ip_address")
        local mac_address = cli("status.sysdetail.ip_addresses.interface[" .. i .. "].mac_address")
        local bytes_rx = cli("status.sysdetail.ip_addresses.interface[" .. i .. "].bytes_rx")
        local bytes_tx = cli("status.sysdetail.ip_addresses.interface[" .. i .. "].bytes_tx")
        local packets_rx = cli("status.sysdetail.ip_addresses.interface[" .. i .. "].packets_rx")
        local packets_tx = cli("status.sysdetail.ip_addresses.interface[" .. i .. "].packets_tx")
        local errors_rx = cli("status.sysdetail.ip_addresses.interface[" .. i .. "].errors_rx")
        local errors_tx = cli("status.sysdetail.ip_addresses.interface[" .. i .. "].errors_tx")
        yml = yml .. "- " .. yaml_string(2, "name", name)
        yml = yml .. yaml_string(3, "mac_address", mac_address)
        yml = yml .. yaml_number(3, "bytes_rx", bytes_rx)
        yml = yml .. yaml_number(3, "bytes_tx", bytes_tx)
        yml = yml .. yaml_number(3, "packets_rx", packets_rx)
        yml = yml .. yaml_number(3, "packets_tx", packets_tx)
        yml = yml .. yaml_number(3, "errors_rx", errors_rx)
        yml = yml .. yaml_number(3, "errors_tx", errors_tx)
    end

    return yml
end

print(device_info())
print(sysdetail())
-----LUA-----
