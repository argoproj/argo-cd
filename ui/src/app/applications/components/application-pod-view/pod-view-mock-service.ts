import {Adjectives, Animals} from './names';
import {Node, Pod, PodStatus} from './pod-view';

function generateInt(max: number): number {
    return Math.floor(Math.random() * Math.floor(max));
}

function generateName(prefix: string) {
    return `${prefix}-${Adjectives[generateInt(Adjectives.length - 1)]}-${Animals[generateInt(Animals.length - 1)]}-${generateInt(1000)}`;
}

function generateNode(): Node {
    const n = generateInt(20);
    return {
        pods: generatePods(n),
        name: generateName('node').toLowerCase(),
        maxCPU: 100,
        maxMem: 1024
    };
}

function generatePods(n: number): Pod[] {
    const pods: Pod[] = [];
    while (n) {
        pods.push({
            name: generateName('pod'),
            status: Object.keys(PodStatus)[generateInt(Object.keys(PodStatus).length)] as PodStatus
        });
        n--;
    }
    return pods;
}

export function GetNodes(): Node[] {
    return [generateNode()];
}
