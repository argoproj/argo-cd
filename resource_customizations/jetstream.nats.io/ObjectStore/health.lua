hs = {}
local healthy_cons = { Created=true, Updated=true, Noop=true, SkipCreate=true, SkipUpdate=true }
if obj.status ~= nil then
    if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
        if condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
        end
        if condition.type == "Ready" and healthy_cons[condition.reason]  then
        hs.status = "Healthy"
        hs.message = condition.message
        return hs
        end
        if condition.type == "Ready" and not healthy_cons[condition.reason] then
        hs.status = "Progressing"
        hs.message = condition.message
        local pattern = "(%d+)%-(%d+)%-(%d+)%a(%d+)%:(%d+)%:([%d]+)%.%d+Z"
        local year, month, day, hour, minute, seconds = condition.lastTransitionTime:match(pattern)
        local event_time = os.time{year = year, month = month, day = day, hour = hour, 
            min = minute, sec = seconds}
        if os.difftime(os.time(), event_time) > 30 then
            hs.status = "Degraded"
            hs.message = "Trying to create resource for more than 30 seconds"
        end
        return hs
        end
    end
    end
end
hs.status = "Progressing"
hs.message = "Waiting for Nacks"
return hs
