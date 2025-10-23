local hs = {}
if obj.status ~= nil then
  if obj.status.status == "Installed" then
    hs.status = "Healthy"
    hs.message = obj.status.message
    return hs
  end
  if obj.status.status == "Failed" or obj.status.status == "Error" then
    hs.status = "Degraded"
    hs.message = obj.status.message
    return hs
  end
end
hs.status = "Progressing"
hs.message = "Waiting for trident installation"
return hs
