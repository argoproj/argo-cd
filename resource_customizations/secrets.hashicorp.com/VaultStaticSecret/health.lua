hs = {}
if obj.status ~= nil then
  if obj.status.secretMAC ~= nil then
    hs.status = "Healthy"
    hs.message = "Secret was found"
    return hs
  end
end

hs.status = "Degraded"
hs.message = "Either the service account cannot authenticate with Vault or requested secret not found - view events for more information"
return hs
