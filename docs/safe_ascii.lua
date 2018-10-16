-----LUA-----
status, result = pcall(cli, "administration.hostnames.hostname")
if status == false then
    print("Command failed")
else
    print("Result: ", result)
end
-----LUA-----