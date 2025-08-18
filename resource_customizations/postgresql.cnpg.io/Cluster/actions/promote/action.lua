local os = require("os")
local instance = actionParams["instance"]
local healthy = obj.status.instancesStatus.healthy
local selected = nil

if instance == "any" then
    -- Select next healthy instance after currentPrimary
    local nextIndex = 0
    for index, node in ipairs(healthy) do
        if node == obj.status.currentPrimary then
            nextIndex = index + 1
            if nextIndex > #healthy then
                nextIndex = 1
            end
            break
        end
    end
    if nextIndex > 0 then
        selected = healthy[nextIndex]
    end
elseif type(instance) == "string" and tonumber(instance) then
    -- Select by instance number
    local suffix = "-" .. instance
    for _, node in ipairs(healthy) do
        if node:sub(-#suffix) == suffix then
            selected = node
            break
        end
    end
elseif type(instance) == "string" then
    -- Select by full name
    for _, node in ipairs(healthy) do
        if node == instance then
            selected = node
            break
        end
    end
end

if selected then
    obj.status.targetPrimary = selected
    obj.status.targetPrimaryTimestamp = os.date("!%Y-%m-%dT%XZ")
    obj.status.Phase = "Switchover in progress"
    obj.status.PhaseReason = "Switching over to " .. selected
else
    error("Could not find a healthy instance matching the criteria: " .. tostring(instance))
end
return obj
