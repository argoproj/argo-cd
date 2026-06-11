local hs = {}

local ingress = {}
if obj.status ~= nil and obj.status.loadBalancer ~= nil and obj.status.loadBalancer.ingress ~= nil then
  ingress = obj.status.loadBalancer.ingress
end

if #ingress > 0 then
  hs.status = "Healthy"
else
  hs.status = "Progressing"
end

return hs
