local health_status = {}
local objectStatus = obj.status
if objectStatus ~= nil then
  local brokersState = objectStatus.brokersState
  if brokersState ~= nil then
    local counter = 0
    local brokerReady = 0
    for i, broker in ipairs(brokersState) do
      local brokerIndex = tonumber(i) - 1
      if (brokerReady <= brokerIndex) then
        brokerReady = brokerIndex+1
      else
        brokerReady = brokerReady
      end
      if broker.configurationState == "ConfigInSync" then
        local cruiseControlState = broker.gracefulActionState.cruiseControlState
        if cruiseControlState == "GracefulUpscaleSucceeded" or cruiseControlState == "GracefulDownscaleSucceeded" then
          counter = counter + 1
        end
      end
    end
    if counter != brokerReady then
      health_status.message = "Broker Config is out of Sync or CruiseControlState is not Ready"
      health_status.status = "Degraded"
      return health_status

    local statusState = objectStatus.state
    local cruiseControlTopicStatus = objectStatus.cruiseControlTopicStatus
    if cruiseControlTopicStatus == "CruiseControlTopicReady" and statusState == "ClusterRunning" then
      health_status.message = "Kafka Brokers, CruiseControl and cluster are in Healthy State."
      health_status.status = "Healthy"
      return health_status
    end
    if cruiseControlTopicStatus == "CruiseControlTopicNotReady" or cruiseControlTopicStatus == nil then
      if statusState == "ClusterReconciling" then
        health_status.message = "Kafka Cluster is Reconciling."
        health_status.status = "Progressing"
        return health_status
      end
      if statusState == "ClusterRollingUpgrading" then
        health_status.message = "Kafka Cluster is Rolling Upgrading."
        health_status.status = "Progressing"
        return health_status
      end
    end

  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for KafkaCluster"
return health_status
