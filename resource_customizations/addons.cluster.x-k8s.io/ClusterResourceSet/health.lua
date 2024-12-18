function getStatus(obj) 
  local hs = {}
  hs.status = "Progressing"
  hs.message = "Initializing cluster resource set"

  if obj.status ~= nil then
    if obj.status.conditions ~= nil then
      for i, condition in ipairs(obj.status.conditions) do

        -- Ready
        if condition.type == "ResourcesApplied" and condition.status == "True" then
          hs.status = "Healthy"
          hs.message = "cluster resource set is applied"
          return hs
        end

        -- Resources Applied
        if condition.type == "ResourcesApplied" and condition.status == "False" then
          hs.status = "Degraded"
          hs.message = condition.message
          return hs
        end

      end
    end
  end
  return hs
end

local hs = getStatus(obj)
return hs