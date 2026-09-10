hs = {status="Unknown", message="Waiting for GameServer to be ready"}

if obj.status then
    local cur  = obj.status.currentState
    local dest = obj.status.desiredState

    -- 1) Check cur and dest status: Progressing
    if cur ~= dest then
    hs.status = "Progressing"
    hs.message = "State change: " .. (cur or "Unknown") .. " → " .. (dest or "Unknown")
    return hs
    end

    -- 2) Check pod: KruisePodReady
    local podCond = obj.status.podStatus or {}
    for _, c in ipairs(podCond.conditions or {}) do
        if c.type == "KruisePodReady" and c.status ~= "True" then
            hs.status = "Degraded"
            hs.message = "Pod is not ready: " .. c.type
            return hs
        end
    end

    -- 3) Both ready: Healthy
    if cur == "Ready" and dest == "Ready" then
    hs.status = "Healthy"
    hs.message = "GameServer is Ready"
    return hs
    end
end

return hs