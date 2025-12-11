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
    elseif #healthy > 0 then
        selected = healthy[1]  -- fallback to first healthy if current primary not healthy
    end
elseif type(instance) == "string" and tonumber(instance) then
    -- Select by instance number
    local wanted = (obj.metadata and obj.metadata.name or "") .. "-" .. instance
    for _, node in ipairs(healthy or {}) do
        if node == wanted then
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
    obj.status.phase = "Switchover in progress"
    obj.status.phaseReason = "Switching over to " .. selected
else
    error("Could not find a healthy instance matching the criteria: " .. instance, 0)
end
return obj
