if obj.spec == nil then
    obj.spec = {}
end
if obj.spec.syncPolicy == nil then
    obj.spec.syncPolicy = {}
end

local automated = obj.spec.syncPolicy.automated
local isEnabled = automated ~= nil and (automated.enabled == true or automated.enabled == nil)

if isEnabled then
    -- Currently ENABLED (enabled=true or enabled=nil): disable it
    automated.enabled = false
else
    -- Currently DISABLED (automated absent or enabled=false): enable it
    if automated == nil then
        obj.spec.syncPolicy.automated = { enabled = true }
    else
        automated.enabled = true
    end
end

return obj
