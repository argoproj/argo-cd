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
if obj.status ~= nil and obj.status.state ~= nil then
    if obj.status.state == "CREATED" and obj.status.connectorState == "RUNNING" and obj.status.failedTasksCount == nil then
        hs.status = "Healthy"
        hs.message = "Connector running"
        return hs
    end
    if obj.status.state == "ERROR" then
        hs.status = "Degraded"
        if obj.status.conditions and #obj.status.conditions > 0 then
            hs.message = obj.status.conditions[1].message -- Kafka Connector only has one condition and nests the issues in the error message here
        else
            hs.message = "No conditions available"
        end
        return hs
    end
    if obj.status.failedTasksCount ~= nil and obj.status.failedTasksCount > 0 then
        hs.status = "Degraded"
        hs.message = "Connector has failed tasks"
        return hs
    end
end
hs.status = "Progressing"
hs.message = "Waiting for Kafka Connector"
return hs