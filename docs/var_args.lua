function seq(...) for k,v in pairs({...}) do print(k,v) end end
function foo(...) for k,v in pairs({...}) do print(k,v) end end

function nothing(...) 
    local s = ""
    for _, v in pairs({...}) do s = s .. v .. "\n" end 
    return s
end

-- function repeats(s, n) return n > 0 and s .. repeats(s, n-1) or "" end
function kvpair(indent, k, v) 
    return string.rep(" ", indent) .. k .. ": " .. v
end

print("foo without args")
foo()
print("foo with one arg")
foo("blaaa")
print("foo with two args")
foo("blaaa", "blubb")
print("foo with seq args")
foo(seq(1,2,3),seq(5,6,7))
print("foo with table args")
print(nothing(kvpair(4, "a","b"),kvpair(4, "c","d")))