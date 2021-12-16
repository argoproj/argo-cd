hs = {}
if obj.status == nil or obj.status.observedGeneration ~= obj.metadata.generation then
  hs.status = "Progressing"
  hs.message = "Waiting for Sync"
  return hs
end
hs.status = "Healthy"
hs.message = "Healthy"
return hs
