-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

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