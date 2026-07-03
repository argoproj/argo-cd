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

hs = {status="Progressing", message="Waiting for GameServerSet initialization"}

if obj.status and obj.metadata.generation == obj.status.observedGeneration then
  local ru = obj.spec.updateStrategy and obj.spec.updateStrategy.rollingUpdate or {}

  -- 1) Pause
  if ru.paused == true then
    hs.status  = "Suspended"
    hs.message = "GameServerSet is paused"
    return hs
  end

  -- 2) Partition
  local partition = ru.partition or 0
  if partition ~= 0 and (obj.status.updatedReplicas or 0) >= (obj.status.replicas or 0) - partition then
    hs.status  = "Suspended"
    hs.message = "Partition=" .. partition .. ", waiting for manual intervention"
    return hs
  end

  -- 3) All updated and ready
  if (obj.status.updatedReadyReplicas or 0) == (obj.status.replicas or 0) then
    hs.status  = "Healthy"
    hs.message = "All GameServerSet replicas are updated and ready"
    return hs
  end

  -- 4) ReadyRelicas not enough
  if (obj.status.readyReplicas or 0) < (obj.status.replicas or 0) then
    hs.status  = "Progressing"
    hs.message = "ReadyReplicas " ..
                 (obj.status.readyReplicas or 0) .. "/" ..
                 (obj.status.replicas or 0) .. ", still progressing"
    return hs
  end
end

return hs