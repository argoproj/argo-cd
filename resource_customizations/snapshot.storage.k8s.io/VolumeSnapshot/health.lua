local hs = {}

if obj.status ~= nil and obj.status.readyToUse then
  hs.status = "Healthy"
  hs.message = "Ready to use"
  return hs
end

if obj.status ~= nil and obj.status.error ~= nil then
  hs.status = "Degraded"
  hs.message = obj.status.error.message
  return hs
end

hs.status = "Progressing"
hs.message = "Waiting for status"

return hs
