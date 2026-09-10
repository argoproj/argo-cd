local hs = {}

-- isAncestorGenerationObserved checks if an ancestor's conditions match the current resource generation
function isAncestorGenerationObserved(obj, ancestor)
  if obj.metadata.generation == nil then
    return true
  end

  if ancestor.conditions == nil or #ancestor.conditions == 0 then
    return false
  end

  for _, condition in ipairs(ancestor.conditions) do
    if condition.observedGeneration ~= nil then
      if condition.observedGeneration ~= obj.metadata.generation then
        return false
      end
    end
  end

  return true
end

if obj.status ~= nil then
  if obj.status.ancestors ~= nil then
    for _, ancestor in ipairs(obj.status.ancestors) do
      if ancestor.conditions ~= nil then
        -- Skip this ancestor if it's not from the current generation
        if not isAncestorGenerationObserved(obj, ancestor) then
          goto continue
        end

        for _, condition in ipairs(ancestor.conditions) do
          if condition.type == "Accepted" and condition.status == "False" then
            hs.status = "Degraded"
            hs.message = condition.message
            return hs
          end
        end

        ::continue::
      end
    end

    -- Only mark as healthy if we found at least one ancestor from the current generation
    for _, ancestor in ipairs(obj.status.ancestors) do
      if ancestor.conditions ~= nil and #ancestor.conditions > 0 then
        if isAncestorGenerationObserved(obj, ancestor) then
          hs.status = "Healthy"
          hs.message = "ClientTrafficPolicy is healthy"
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for ClientTrafficPolicy status"
return hs
