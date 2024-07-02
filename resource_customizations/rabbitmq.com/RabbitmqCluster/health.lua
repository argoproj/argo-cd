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

    if clusterAvailable.status == "Unknown" or allReplicasReady.status == "Unknown" then
      hs.status = "Degraded"
      hs.message = "No statefulset or endpoints found"
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