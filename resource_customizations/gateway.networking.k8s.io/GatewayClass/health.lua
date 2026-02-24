local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Accepted" then
      -- Treat stale conditions (observedGeneration != current generation) as not-yet-reconciled.
      if obj.metadata ~= nil and obj.metadata.generation ~= nil and condition.observedGeneration ~= nil then
        if condition.observedGeneration ~= obj.metadata.generation then
          goto continue
        end
      end

      if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message or "GatewayClass is accepted"
        return hs
      elseif condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message or "GatewayClass is not accepted"
        return hs
      else
        hs.status = "Progressing"
        hs.message = condition.message or "Waiting for GatewayClass status"
        return hs
      end
    end
    ::continue::
  end
end

hs.status = "Progressing"
hs.message = "Waiting for GatewayClass status"
return hs
