local adopted = { status = "Unknown" }
local advertised = { status = "Unknown" }
local discovered = { status = "Unknown" }

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, c in ipairs(obj.status.conditions) do
      if c.type == "Adopted" then
        adopted = c
      elseif c.type == "Advertised" then
        advertised = c
      elseif c.type == "Discoverable" then
        discovered = c
      end
    end
  end
end

if adopted.status == "False" then
  return { status = "Degraded", message = adopted.message }
elseif advertised.reason == "AdvertiseError" or advertised.reason == "UnadvertiseError" then
  return { status = "Degraded", message = advertised.message }
elseif discovered.reason == "DiscoveryError" then
  return { status = "Unknown", message = discovered.message }
elseif discovered.status == "True" then
  return { status = "Healthy", message = discovered.message }
else
  return { status = "Progressing", message = discovered.message }
end
