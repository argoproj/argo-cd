import {Select} from 'argo-ui';
import * as React from 'react';
import {Spacer} from '../../../shared/components/spacer';

export type ContainerGroup = {offset: number; containers: string[]};
export const ContainerSelector = ({
    containerGroups,
    containerName,
    onClickContainer
}: {
    containerGroups?: ContainerGroup[];
    containerName: string;
    onClickContainer: (group: ContainerGroup, index: number, logs: string) => void;
}) => {
    if (!containerGroups) {
        return <></>;
    }
    const containers = containerGroups?.reduce((acc, group) => acc.concat(group.containers), []);
    const containerNames = containers?.map(container => container.name);
    const containerGroup = (n: string) => {
        return containerGroups.find(group => group.containers.find(container => container === n));
    };
    const containerIndex = (n: string) => {
        return containerGroup(n).containers.findIndex(container => container === n);
    };
    return (
        <>
            <label>For</label>
            <Spacer />
            <Select value={containerName} onChange={option => onClickContainer(containerGroup(option.value), containerIndex(option.value), 'logs')} options={containerNames} />
        </>
    );
};
