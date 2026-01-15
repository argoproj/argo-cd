hs = {
    status = "Progressing",
    message = "Update in progress"
}
if (obj.metadata.labels ~= nil and obj.metadata.labels["shoot.gardener.cloud/status"] ~= nil) then
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "healthy" then
        hs.status = "Healthy"
        hs.message = "Component state: Healthy"
    end
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "progressing" then
        hs.status = "Progressing"
        hs.message = "Component state: Update in progress"
    end
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "unhealthy" then
        if obj.status ~= nil then
            if obj.status.conditions ~= nil then
                for i, condition in ipairs(obj.status.conditions) do
                    if condition.status ~= "True" then
                      -- calculate how long the "unhealthy" state is set - this prevents us from unnecessary alerts
                      offsetSeconds = 10000 -- default: very long
                      if condition.lastTransitionTime ~= nil then
                        local year, month, day, hour, min, sec = string.match(condition.lastTransitionTime, "(%d+)-(%d+)-(%d+)T(%d+):(%d+):(%d+)Z")
                        lastTransitionTime = os.time({year=year, month=month, day=day, hour=hour, min=min, sec=sec})
                        nowString = os.date("!%Y-%m-%dT%H:%M:%SZ") 
                        year, month, day, hour, min, sec = string.match(nowString, "(%d+)-(%d+)-(%d+)T(%d+):(%d+):(%d+)Z")
                        now = os.time({year=year, month=month, day=day, hour=hour, min=min, sec=sec})
                        offsetSeconds = now - lastTransitionTime
                      end
                      if offsetSeconds < 900 then
                        hs.status = "Progressing"
                        hs.message = "Component state: Reconcile - status unhealthy (< 15 minutes)"
                        return hs
                      else
                        hs.status = "Degraded"
                        hs.message = "Component state: Unhealthy (> 15 minutes)"
                        return hs
                      end
                    end
                end
            end
        end
        hs.status = "Degraded"
        hs.message = "Component state: Unhealthy"
    end
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "unknown" then
        hs.status = "Unknown"
        hs.message = "Component state: Unknown"
    end
    return hs
end
return hs
