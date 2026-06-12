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

-- There is no value in the manifest that can lead to conclude that
-- this resource is in a "Degraded" state. Update this, if in the future
-- this possibility arises.

if obj.status == nil or obj.status.solrNodes == nil then
  return {
    status = "Progressing",
    message = "Waiting for solr to exist",
  }
end

for _, solrNode in ipairs(obj.status.solrNodes) do
  if not solrNode.ready then
    return {
      status = "Progressing",
      message = "Not all replicas are ready",
    }
  end
end

return {
  status = "Healthy",
  message = "Solr is ready",
}