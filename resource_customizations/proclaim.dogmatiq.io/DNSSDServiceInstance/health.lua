-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

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
