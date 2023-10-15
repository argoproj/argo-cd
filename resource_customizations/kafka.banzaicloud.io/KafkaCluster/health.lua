local health_status = {}
if obj.status ~= nil then
  if obj.status.brokersState ~= nil then
    local numberBrokers = #obj.status.brokersState
    local healthyBrokers = 0
    for i, broker in pairs(obj.status.brokersState) do
      if broker.configurationState == "ConfigInSync" and broker.gracefulActionState.cruiseControlState == "GracefulUpscaleSucceeded" then
        healthyBrokers = healthyBrokers + 1
      end
      if broker.configurationState == "ConfigInSync" and broker.gracefulActionState.cruiseControlState == "GracefulDownscaleSucceeded" then
        healthyBrokers = healthyBrokers + 1
      end
    end
    if numberBrokers == healthyBrokers then
      if obj.status.cruiseControlTopicStatus == "CruiseControlTopicReady" and obj.status.state == "ClusterRunning" then
        health_status.message = "Kafka Brokers, CruiseControl and cluster are in Healthy State."
        health_status.status = "Healthy"
        return health_status
      end
      if obj.status.cruiseControlTopicStatus == "CruiseControlTopicNotReady" or obj.status.cruiseControlTopicStatus == nil then
        if obj.status.state == "ClusterReconciling" then
          health_status.message = "Kafka Cluster is Reconciling."
          health_status.status = "Progressing"
          return health_status
        end
        if obj.status.state == "ClusterRollingUpgrading" then
          health_status.message = "Kafka Cluster is Rolling Upgrading."
          health_status.status = "Progressing"
          return health_status
        end
      end
    else
      health_status.message = "Broker Config is out of Sync or CruiseControlState is not Ready"
      health_status.status = "Degraded"
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for KafkaCluster"
return health_status