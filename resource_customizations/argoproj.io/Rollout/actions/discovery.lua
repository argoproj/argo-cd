actions = {}
actions["resume"] = {["available"] = false}

local paused = false


if obj.status ~= nil and obj.status.verifyingPreview ~= nil then
    print("vp")
    paused = obj.status.verifyingPreview
elseif obj.spec.paused ~= nil then
    print("paused")
    print(obj.spec.paused)
    paused = obj.spec.paused
end
print(paused)
if paused then
    print("inside")
    actions["resume"]["available"] = true
end


print(actions.resume.available)
return actions
