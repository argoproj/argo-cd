local hs = {}

if obj.status ~= nil then
  if obj.status.ancestors ~= nil then
    for _, ancestor in ipairs(obj.status.ancestors) do
      if ancestor.conditions ~= nil then
        for _, condition in ipairs(ancestor.conditions) do
          if condition.type == "Accepted" then
            if condition.observedGeneration ~= nil and condition.observedGeneration ~= obj.metadata.generation then
              hs.status = "Progressing"
              hs.message = "Waiting for EnvoyExtensionPolicy status to be updated"
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
  end
end

hs.status = "Progressing"
hs.message = "Waiting for EnvoyExtensionPolicy status"
return hs
