local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local ready = false
    local synced = false
    local suspended = false
    for i, condition in ipairs(obj.status.conditions) do

      if condition.type == "Ready" then
        ready = condition.status == "True"
        ready_message = condition.reason
      elseif condition.type == "Synced" then
        synced = condition.status == "True"
        if condition.reason == "ReconcileError" then
          synced_message = condition.message
        elseif condition.reason == "ReconcilePaused" then
          suspended = true
          suspended_message = condition.reason
        end
      end
    end
    if ready and synced then
      hs.status = "Healthy"
      hs.message = ready_message
    elseif synced == false and suspended == true then
      hs.status = "Suspended"
      hs.message = suspended_message
    elseif ready == false and synced == true and suspended == false then
      hs.status = "Progressing"
      hs.message = "Waiting for DBCluster to be available"
    else
      hs.status = "Degraded"
      hs.message = synced_message
    end
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for DBCluster to be created"
return hs
