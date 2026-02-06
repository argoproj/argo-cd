local hs = {}
if obj.spec.suspend ~= nil and obj.spec.suspend == true then
  hs.message = obj.kind .. " is suspended"
  hs.status = "Suspended"
  return hs
end
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local numFailing = 0
    local numSucceeded = 0
    local message = ""
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        if condition.status == "True" then
          numSucceeded = numSucceeded + 1
        elseif condition.status == "False" then
          numFailing = numFailing + 1
        end
        message = condition.reason
      elseif condition.type == "Reconciling" and condition.status == "True" then
        numFailing = numFailing + 1
      end
    end
    if(numFailing == 2) then
      hs.message = message
      hs.status = "Degraded"
      return hs
    elseif(numSucceeded == 1) then
      hs.message = message
      hs.status = "Healthy"
      return hs
    else
      hs.message = message
      hs.status = "Degraded"
      return hs
    end
  end
end
hs.message = "Status unknown"
hs.status = "Progressing"
return hs
