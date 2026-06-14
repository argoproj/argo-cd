local hs = {}
if obj.status ~= nil then
  if obj.status.phase ~= nil then
    if obj.status.phase == "RUNNING" then
      hs.status = "Healthy"
      hs.message = "Connect running"
      return hs
    end
    if obj.status.phase == "PROVISIONING" then
      hs.status = "Progressing"
      hs.message = "Connect provisioning"
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Connect"
return hs
