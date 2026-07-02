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

hs = {}
clusterAvailable = {}
allReplicasReady = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then

    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "ReconcileSuccess" and condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
      if condition.type == "ClusterAvailable" then
        clusterAvailable.status = condition.status
        clusterAvailable.message = condition.message
      end
      if condition.type == "AllReplicasReady" then
        allReplicasReady.status = condition.status
        allReplicasReady.message = condition.message
      end
    end

    -- Treat transient/initial 'Unknown' condition as Progressing instead of Degraded.
    -- The RabbitMQ operator sets these conditions to Unknown briefly while forming the cluster,
    -- so mapping Unknown->Progressing prevents false Degraded states during normal reconciliation.
    if clusterAvailable.status == "Unknown" or allReplicasReady.status == "Unknown" then
      hs.status = "Progressing"
      hs.message = "Waiting for RabbitMQ cluster readiness (conditions unknown)"
      return hs
    end

    if clusterAvailable.status == "False" then
      hs.status = "Progressing"
      hs.message = "Waiting for RabbitMQ cluster formation"
      return hs
    end

    if allReplicasReady.status == "False" then
      hs.status = "Progressing"
      hs.message = "Waiting for RabbitMQ instances ready"
      return hs
    end

    if clusterAvailable.status == "True" and allReplicasReady.status == "True" then
      hs.status = "Healthy"
      hs.message = "RabbitMQ cluster ready"
      return hs
    end

  end
end

hs.status = "Progressing"
hs.message = "Waiting for RabbitMQ Operator"
return hs