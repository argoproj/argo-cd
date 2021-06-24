hs = {}
if obj.status ~= nil then
  if obj.status.status == "Installed" then
    hs.status = "Healthy"
    hs.message = obj.status.message
    return hs
  end
end
hs.status = "Progressing"
hs.message = "Waiting for trident installation"
return hs
