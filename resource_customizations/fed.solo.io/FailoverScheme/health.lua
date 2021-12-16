hs = {}
if obj.status == nil or obj.status.state == "PENDING" or obj.status.state == "PROCESSING" then
  hs.status = "Progressing"
  hs.message = "Waiting for Gloo"
  return hs
end
if obj.status.state == "ACCEPTED" then
  hs.status = "Healthy"
  hs.message = "Healthy"
  return hs
end
hs.status = "Degraded"
hs.message = obj.status.message
return hs
