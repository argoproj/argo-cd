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
      hs.message = obj.status.description
    elseif obj.status.currentStatus ~= "invalid" then
      hs.status = "Degraded"
      hs.message = obj.status.description
    end
  end
end

return hs