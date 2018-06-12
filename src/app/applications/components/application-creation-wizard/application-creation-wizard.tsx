import { MockupList } from 'argo-ui';
import * as path from 'path';
import * as React from 'react';
import { BehaviorSubject } from 'rxjs';

import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

import { AppParams, AppsList, EnvironmentsList, NewAppParams, RepositoryList } from './steps';

export { NewAppParams } from './steps';

enum Step { SelectRepo = 0, SelectApp = 1, SelectEnvironments = 2, SetParams = 3 }
interface StepInfo { title: string; canNext(): boolean; next(): any; render(): React.ReactNode; canPrev(): boolean; prev(): any; }

interface State {
    repos: models.Repository[];
    clusters: models.Cluster[];
    apps: models.KsonnetAppSpec[];
    envs: { [key: string]: models.KsonnetEnvironment; };
    selectedRepo: string;
    selectedApp: models.KsonnetAppSpec;
    selectedEnv: string;
    appParams: NewAppParams;
    appParamsValid: boolean;
    step: Step;
    loading: boolean;
}

export interface WizardProps { onStateChanged: (state: WizardStepState) => any; createApp: (params: NewAppParams) => any; }

export interface WizardStepState { nextTitle: string; next?: () => any; prev?: () => any; }

export class ApplicationCreationWizardContainer extends React.Component<WizardProps, State> {

    private submitAppParamsForm = new BehaviorSubject<any>(null);

    constructor(props: WizardProps) {
        super(props);
        this.state = {
            apps: [],
            clusters: [],
            repos: [],
            envs: {},
            selectedApp: null,
            selectedRepo: null,
            selectedEnv: null,
            appParamsValid: false,
            step: Step.SelectRepo,
            loading: false,
            appParams: null,
        };
        this.notifyStepStateChanged();
    }

    public async componentDidMount() {
        const [repos, clusters] = await Promise.all([services.reposService.list(), services.clustersService.list()]);
        this.setState({repos, clusters});
    }

    public render() {
        const currentStep = this.getCurrentStep();

        return (
            <div>
                <h4>{currentStep.title}</h4>
                {currentStep.render()}
            </div>
        );
    }

    private getCurrentStep(): StepInfo {
        switch (this.state.step) {
            case Step.SelectApp:
                return {
                    title: 'Select application',
                    canNext: () => !!this.state.selectedApp,
                    next: () => this.updateState({ envs: this.state.selectedApp.environments, step: Step.SelectEnvironments}),
                    canPrev: () => true,
                    prev: () => this.updateState({ step: Step.SelectRepo }),
                    render: () => this.state.apps ? (
                        <AppsList apps={this.state.apps} selectedApp={this.state.selectedApp} onAppSelected={(app) => this.updateState({ selectedApp: app })}/>
                    ) : (
                        <MockupList height={50} marginTop={10}/>
                    ),
                };
            case Step.SelectEnvironments:
                return {
                    title: 'Select environment',
                    canNext: () => !!this.state.selectedEnv,
                    next: async () => {
                        const selectedEnv = this.state.envs[this.state.selectedEnv];
                        this.updateState({
                            appParams: {
                                applicationName: `${this.state.selectedApp.name}-${this.state.selectedEnv}`,
                                repoURL: this.state.selectedRepo,
                                environment: this.state.selectedEnv,
                                clusterURL: selectedEnv.destination.server,
                                namespace: selectedEnv.destination.namespace,
                                path: path.dirname(this.state.selectedApp.path),
                            }, step: Step.SetParams,
                        });
                    },
                    canPrev: () => true,
                    prev: () => this.updateState({ step: Step.SelectRepo }),
                    render: () => (
                        <EnvironmentsList envs={this.state.envs} selectedEnv={this.state.selectedEnv} onEnvsSelectionChanged={(env) => this.updateState({ selectedEnv: env })}/>
                    ),
                };
            case Step.SetParams:
                return {
                    title: 'Review application parameters',
                    canNext: () => this.state.appParamsValid,
                    next: async () => this.submitAppParamsForm.next({}),
                    canPrev: () => true,
                    prev: () => this.updateState({ step: Step.SelectEnvironments }),
                    render: () => (
                        <AppParams
                            appParams={this.state.appParams}
                            submitForm={this.submitAppParamsForm}
                            onSubmit={this.props.createApp}
                            onValidationChanged={(isValid) => this.updateState({ appParamsValid: isValid })} />
                    ),
                };
            case Step.SelectRepo:
            default:
                return {
                    title: 'Select repository',
                    canNext: () => !!this.state.selectedRepo,
                    next: async () => {
                        try {
                            this.updateState({ loading: true, step: Step.SelectApp, apps: null });
                            const apps = await services.reposService.ksonnetApps(this.state.selectedRepo);
                            this.updateState({ apps, loading: false });
                        } finally {
                            this.updateState({ loading: false });
                        }
                    },
                    canPrev: () => false,
                    prev: null,
                    render: () => (
                        <RepositoryList
                            selectedRepo={this.state.selectedRepo}
                            repos={this.state.repos}
                            onSelectedRepo={(repo) => this.updateState({ selectedRepo: repo })}/>
                    ),
                };
        }
    }

    private updateState<K extends keyof State>(update: (Pick<State, K> | State)) {
        this.setState(update, () => {
            this.notifyStepStateChanged();
        });
    }

    private notifyStepStateChanged() {
        const currentStep = this.getCurrentStep();
        this.props.onStateChanged({
            next: currentStep.canNext() && !this.state.loading && currentStep.next,
            prev: currentStep.canPrev()  && !this.state.loading && currentStep.prev,
            nextTitle: this.state.step === Step.SetParams ? 'Create' : 'Next',
        });
    }
}
