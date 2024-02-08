local hs = {}

if obj.status ~= nil and (obj.status.health ~= nil or obj.status.expectedNodes ~= nil) then
  if obj.status.health == "red" then
    hs.status = "Degraded"
    hs.message = "Elastic Beat status is Red"
    return hs
  elseif obj.status.health == "green" then
    hs.status = "Healthy"
    hs.message = "Elastic Beat status is Green"
    return hs
  elseif obj.status.health == "yellow" then
    if obj.status.availableNodes ~= nil and obj.status.expectedNodes ~= nil then
        hs.status = "Progressing"
        hs.message = "Elastic Beat status is deploying, there is " .. obj.status.availableNodes .. " instance(s) on " .. obj.status.expectedNodes .. " expected"
        return hs
    else
        hs.status = "Progressing"
        hs.message = "Elastic Beat phase is progressing"
        return hs
    end
  elseif obj.status.health == nil then
    hs.status = "Progressing"
    hs.message = "Elastic Beat phase is progressing"
    return hs
  end
end

hs.status = "Unknown"
hs.message = "Elastic Beat status is unknown. Ensure your ArgoCD is current and then check for/file a bug report: https://github.com/argoproj/argo-cd/issues"
return hs
