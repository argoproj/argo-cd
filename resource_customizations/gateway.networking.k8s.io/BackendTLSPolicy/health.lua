local hs = {}

hs.status = "Progressing"
hs.message = "Waiting for BackendTLSPolicy status"

if obj.status ~= nil and obj.status.ancestors ~= nil then
  for i, ancestor in ipairs(obj.status.ancestors) do
    if ancestor.conditions ~= nil then
      for j, condition in ipairs(ancestor.conditions) do
        if condition.type == "Accepted" then
          if condition.status ~= "True" then
            hs.status = "Degraded"
            hs.message = "Ancestor " .. (ancestor.ancestorRef.name or "") .. ": " .. condition.message
            return hs
          else
            hs.status = "Healthy"
            hs.message = "BackendTLSPolicy is healthy"
          end
        end

        if condition.type == "ResolvedRefs" then
          if condition.status ~= "True" then
            hs.status = "Degraded"
            hs.message = "Ancestor " .. (ancestor.ancestorRef.name or "") .. ": " .. condition.message
            return hs
          end
        end
      end
    end
  end
end

return hs
