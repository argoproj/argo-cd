-- Status reporting information detailed here
-- https://projectcontour.io/docs/main/config/fundamentals/#status-reporting
hs = {}

if obj.status ~= nil then
  if obj.status.currentStatus ~= nil then
    if obj.status.currentStatus == "valid" then
      hs.status = "Healthy"
    else
      hs.status = "Degraded"
    end
    hs.message = obj.status.description
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for status"
return hs
