hs = {}

if obj.spec.suspend == true then
    -- Set to Healthy instead of Suspended until bug is resolved
    -- See https://github.com/argoproj/argo-cd/issues/24428
    hs.status = "Healthy"
    hs.message = "CronJob is Suspended"
    return hs
end

if obj.status ~= nil then
    if obj.status.lastScheduleTime ~= nil then

        -- Job is running its first execution and has not yet reported any success
        if obj.status.lastSuccessfulTime == nil then
            -- Set to healthy even if it may be degraded, because we don't know
            -- if it was not yet executed or if it never succeeded
            hs.status = "Healthy"
            hs.message = "The CronJob never completed successfully. It may not be healthy"
            return hs
        end


        -- Job is progressing, so lastScheduleTime will always be greater than lastSuccessfulTime
        -- Set to healthy since we do not know if it is Degraded
        -- See https://github.com/argoproj/argo-cd/issues/24429
        if obj.status.active ~= nil and table.getn(obj.status.active) > 0 then
            hs.status = "Healthy"
            hs.message = "The job is running. Its last execution may not have been successful"
            return hs
        end

    -- If the CronJob has no active jobs and the lastSuccessfulTime < lastScheduleTime
    -- then we know it failed the last execution
        if obj.status.lastSuccessfulTime ~= nil and obj.status.lastSuccessfulTime < obj.status.lastScheduleTime then
            hs.status = "Degraded"
            hs.message = "CronJob has not completed its last execution successfully"
            return hs
        end

        hs.message = "CronJob has completed its last execution successfully"
        hs.status = "Healthy"
        return hs
    end

    -- There is no way to know if as CronJob missed its execution based on status
    -- so we assume Healthy even if a cronJob is not getting scheduled.
    -- https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#job-creation
    hs.message = "CronJob has not been scheduled yet"
    hs.status = "Healthy"
    return hs
end

hs.status = "Progressing"
hs.message = "Waiting for CronJob"
return hs
