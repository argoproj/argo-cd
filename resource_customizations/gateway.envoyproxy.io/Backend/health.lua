local hs = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Accepted" then
        if condition.observedGeneration ~= nil and condition.observedGeneration ~= obj.metadata.generation then
          hs.status = "Progressing"
          hs.message = "Waiting for Backend status to be updated"
          return hs
        end
        if condition.status == "True" then
          hs.status = "Healthy"
          hs.message = condition.message
          return hs
        else
          hs.status = "Degraded"
          hs.message = condition.message
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Backend status"
return hs
