health_status = {}
if obj.status ~= nil then
  if obj.status.state ~= nil then
    if obj.status.state == "running" then
      health_status.status = "Healthy"
      health_status.message = "Install of Apigee Deployment is done"
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "An install plan for an Apigee Deployment is pending installation"
return health_status