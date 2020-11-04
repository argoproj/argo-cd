import {Node, Pod, PodPhase, HealthStatuses, HealthStatusCode} from '../../../shared/models';
import {Adjectives, Animals} from './names';

const podStatusWeights = {
    PodPending: 10,
    PodRunning: 10,
    PodSucceeded: 80,
    PodFailed: 5,
    PodUnknown: 5
};

const podHealthWeights = {
    Unknown: 0,
    Progressing: 5,
    Suspended: 5,
    Healthy: 70,
    Degraded: 20,
    Missing: 0
};

function generateInt(min: number, max: number): number {
    return Math.floor(min + Math.random() * Math.floor(max + 1 - min));
}

function generateName(prefix: string) {
    return `${prefix}-${Adjectives[generateInt(0, Adjectives.length - 1)]}-${Animals[generateInt(0, Animals.length - 1)]}-${generateInt(100, 999)}`;
}

function generateNode(): Node {
    const name = generateName('node');
    const pods = generatePods(generateInt(10, 25), name);
    return {
        metadata: {labels: {'kubernetes.io/hostname': name}},
        status: {
            nodeInfo: {
                operatingSystem: 'linux',
                architecture: 'amd64',
                kernelVersion: '4.19.76-linuxkit'
            },
            capacity: {
                'cpu': 6,
                'memory': 1024,
                'storage': 60000,
                'ephemeral-storage': 5000
            },
            allocatable: {
                'cpu': generateInt(0, 6),
                'memory': generateInt(0, 1024),
                'storage': generateInt(0, 60000),
                'ephemeral-storage': generateInt(0, 5000)
            }
        },
        pods
    };
}

function getValuesFromWeights(weights: number[], values: string[]): string {
    const sum = weights.reduce((acc, el) => acc + el, 0);
    let accumulator = 0;
    weights = weights.map(item => (accumulator = item + accumulator));
    const rand = Math.random() * sum;
    return values[weights.filter(el => el <= rand).length];
}

function podSort(a: Pod, b: Pod): number {
    return a.metadata.name < b.metadata.name ? -1 : 1;
}

function generatePods(n: number, nodeName: string): Pod[] {
    const pods: Pod[] = [];
    while (n) {
        pods.push({
            fullName: '',
            metadata: {name: generateName('pod').toLowerCase()},
            spec: {nodeName},
            status: {
                phase: getValuesFromWeights(Object.values(podStatusWeights), Object.values(PodPhase)) as PodPhase,
                message: ''
            },
            health: getValuesFromWeights(Object.values(podHealthWeights), Object.values(HealthStatuses)) as HealthStatusCode
        });
        n--;
    }
    pods.sort(podSort);
    return pods;
}

export function GetNodes(x: number): Node[] {
    const nodes: Node[] = Array(x);
    for (let i = 0; i < x; i++) {
        nodes[i] = generateNode();
    }
    console.log(nodes);
    return nodes;
}
