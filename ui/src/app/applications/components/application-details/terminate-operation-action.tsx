import {NotificationType} from 'argo-ui';
import * as React from 'react';
import {ErrorNotification} from '../../../shared/components';
import {ContextApis} from '../../../shared/context';
import * as appModels from '../../../shared/models';
import {services} from '../../../shared/services';

// Returns the Terminate action for the application more menu. The action is only offered while an operation is Running.
export const terminateOperationMenuItems = (app: appModels.Application, appContext: ContextApis, title: React.ReactNode) =>
    app.status?.operationState?.phase === 'Running'
        ? [
              {
                  iconClassName: 'fa fa-stop-circle',
                  title,
                  action: async () => {
                      const confirmed = await appContext.popup.confirm('Terminate operation', 'Are you sure you want to terminate operation?');
                      if (!confirmed) return;
                      try {
                          await services.applications.terminateOperation(app.metadata.name, app.metadata.namespace);
                      } catch (e) {
                          appContext.notifications.show({
                              content: <ErrorNotification title='Unable to terminate operation' e={e} />,
                              type: NotificationType.Error
                          });
                      }
                  }
              }
          ]
        : [];
