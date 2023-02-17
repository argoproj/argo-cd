import {DropDownMenu} from 'argo-ui';
import * as React from 'react';
import {Button} from '../../../shared/components/button';

type ContainerGroup = {offset: number; containers: string[]};
export const ContainerSelector = ({
    containerGroups,
    containerName,
    onClickContainer
}: {
    containerGroups: ContainerGroup[];
    containerName: string;
    onClickContainer: (group: ContainerGroup, index: number, logs: string) => void;
}) => {
    const containerItems: {title: any; action: () => any}[] = [];
    if (containerGroups?.length > 0) {
        containerGroups.forEach(group => {
            containerItems.push({
                title: group.offset === 0 ? 'CONTAINER' : 'INIT CONTAINER',
                action: null
            });

            group.containers.forEach((container: any, index: number) => {
                const title = (
                    <div className='d-inline-block'>
                        {container.name === containerName && <i className='fa fa-angle-right' />}
                        <span title={container.name} className='container-item'>
                            {container.name}
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
    return (
        containerGroups?.length > 0 && (
            <DropDownMenu
                anchor={() => (
                    <Button icon='stream' title='Containers'>
                        {containerName.padEnd(5, ' ').substr(0, 4)}...
                    </Button>
                )}
                items={containerItems}
            />
        )
    );
};
