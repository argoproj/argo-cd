local health_status = {}
if obj.status ~= nil then
  if obj.status.readyReplicas ~= 0 and obj.status.readyReplicas == obj.status.replicas then
    health_status.status = "Healthy"
    health_status.message = "All ZK Nodes have joined the ensemble"
    return health_status
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for ZK Nodes to join the ensemble"
return health_status