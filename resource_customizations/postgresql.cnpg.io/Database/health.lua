local hs = {}

if obj.status ~= nil and ob.status.applied ~= nil then
  if obj.status.applied then
    hs.status = "Healthy"
    if obj.status.message ~= nil then
      hs.message = obj.status.message
    else
      hs.message = "Database has been created"
    end
    return hs
  else
    hs.status = "Degraded"
    if obj.status.message ~= nil then
      hs.message = obj.status.message
    else
      hs.message = "Database has not been created"
    end
    return hs
  end
end

hs.status = "Unknown"
hs.message = "Database status unknown"
return hs
