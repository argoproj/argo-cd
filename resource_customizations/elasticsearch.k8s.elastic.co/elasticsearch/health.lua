hs = {}
if obj.status ~= nil then
  if obj.status.phase == "Ready" then
    if obj.status.health == "green" then
      hs.status = "Healthy"
      hs.message = "Elasticsearch Cluster status is Green"
      return hs
    elseif obj.status.health == "yellow" then
      hs.status = "Degraded"
      hs.message = "Elasticsearch Cluster status is Yellow"
      return hs
    elseif obj.status.health == "red" then
      hs.status = "Degraded"
      hs.message = "Elasticsearch Cluster status is Red"
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

hs.status = "Unknown"
hs.message = "Elasticsearch Cluster status is unknown"
return hs