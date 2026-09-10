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