import {DropDownMenu, Tooltip} from "argo-ui";
import * as React from "react";

type ContainerGroup = { offset: number, containers: String[] };
export const ContainerSelector = ({
                                      containerGroups,
                                      containerName,
                                      onClickContainer
                                  }: { containerGroups: ContainerGroup[], containerName: string, onClickContainer: (group: ContainerGroup, index: number, logs: String) => void }) => {
    const containerItems: { title: any; action: () => any }[] = [];
    if (containerGroups?.length > 0) {
        containerGroups.forEach(group => {
            containerItems.push({
                title: group.offset === 0 ? 'CONTAINER' : 'INIT CONTAINER',
                action: null
            });

            group.containers.forEach((container: any, index: number) => {
                const title = (
                    <div className='d-inline-block'>
                        {container.name === containerName && <i className='fa fa-angle-right'/>}
                        <span title={container.name} className='container-item'>
                            {container.name.toUpperCase()}
                        </span>
                    </div>
                );
                containerItems.push({
                    title,
                    action: () => (container.name === containerName ? {} : onClickContainer(group, index, 'logs'))
                });
            });
        });
    }
    return containerGroups?.length > 0 && (
        <DropDownMenu
            anchor={() => (
                <Tooltip content='Containers'>
                    <button className='argo-button argo-button--base'>
                        <i className='fa fa-stream'/>
                    </button>
                </Tooltip>
            )}
            items={containerItems}
        />
    )
}