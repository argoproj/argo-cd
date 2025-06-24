local hs = {}


if obj.metadata.annotations ~= nil and obj.metadata.annotations["cluster.x-k8s.io/paused"] ~= nil then
    hs.status = "Suspended"
    hs.message = "KubeadmControlPlane is paused"
    return hs
end

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        if condition.status == "False" then
          if condition.reason == "RollingUpdateInProgress" then
            hs.status = "Progressing"
          elseif condition.reason == "ScalingDown" then
            hs.status = "Progressing"
          else
            hs.status = "Degraded"
          end
          hs.message = condition.message
          return hs
        else
          hs.status = "Healthy"
          hs.message = "KubeadmControlPlane is ready"
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for control planes"
return hs