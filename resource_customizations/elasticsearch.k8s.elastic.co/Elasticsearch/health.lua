-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

local hs = {}
if obj.status ~= nil then
  if obj.status.availableNodes ~= nil then
    local sum = 0
    for _,node in pairs(obj.spec.nodeSets) do
      sum = sum + node.count
    end
    if obj.status.availableNodes < sum then
      hs.status = "Progressing"
      hs.message = "The desired amount of availableNodes is " .. sum .. " but the current amount is " .. obj.status.availableNodes
      return hs
    elseif obj.status.availableNodes == sum then
      if obj.status.phase ~= nil and obj.status.health ~= nil then
        if obj.status.phase == "Ready" then
          if obj.status.health == "green" then
            hs.status = "Healthy"
            hs.message = "Elasticsearch Cluster status is Green"
            return hs
          elseif obj.status.health == "yellow" then
            hs.status = "Progressing"
            hs.message = "Elasticsearch Cluster status is Yellow. Check the status of indices, replicas and shards"
            return hs
          elseif obj.status.health == "red" then
            hs.status = "Degraded"
            hs.message = "Elasticsearch Cluster status is Red. Check the status of indices, replicas and shards"
            return hs
          end
        elseif obj.status.phase == "ApplyingChanges" then
          hs.status = "Progressing"
          hs.message = "Elasticsearch phase is ApplyingChanges"
          return hs
        elseif obj.status.phase == "MigratingData" then
          hs.status = "Progressing"
          hs.message = "Elasticsearch phase is MigratingData"
          return hs
        elseif obj.status.phase == "Invalid" then
          hs.status = "Degraded"
          hs.message = "Elasticsearch phase is Invalid"
          return hs
        end
      end
    end
  end
end

hs.status = "Unknown"
hs.message = "Elasticsearch Cluster status is unknown. Ensure your ArgoCD is current and then check for/file a bug report: https://github.com/argoproj/argo-cd/issues"
return hs
