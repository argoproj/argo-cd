-- Status reporting information detailed here
-- https://projectcontour.io/docs/main/config/fundamentals/#status-reporting
hs = {
  status = "Progressing",
  message = "Waiting for status",
}

if obj.status ~= nil then
  if obj.status.currentStatus ~= nil then
    if obj.status.currentStatus == "valid" then
      hs.status = "Healthy"
    elseif obj.status.currentStatus ~= "NotReconciled" then
      hs.status = "Degraded"
    end
    hs.message = obj.status.description
  end
end

return hs