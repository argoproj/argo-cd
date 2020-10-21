import * as React from 'react';
require('./ui-banner.scss');

export interface UiBannerProps extends React.Props<any> {
    items: {apiUrl:string, fields: Array<string> };
}


export class Banner extends React.Component<UiBannerProps>{
    state:any = {
        bannerVisible:false,
        isLoaded:false,
        apiData:[]
    }
    constructor(props:UiBannerProps){
        super(props)
    }
    setBanner = () => {
        this.setState({
        bannerVisible:false,
        isLoaded:false
        })
    }
    async componentDidMount(){
        fetch(this.props.items.apiUrl)
        .then(response => response.json())
        .then((data) =>  this.setState({apiData:data,bannerVisible:true,isLoaded:true}),
        (error)=>{
            console.log("Unable to fetch json for ui-banner",error)
        });
    
    }
    render(){
        return (
            <div className='ui_banner' style={{visibility: this.state.bannerVisible? "visible": "hidden"}} >
                <button className='ui_banner__close' aria-hidden='true' onClick={this.setBanner}>
                    <span>
                        <i className='argo-icon-close' aria-hidden='true'/>
                    </span>
                </button>
                <div className="ui_banner__text">
                    {this.state.isLoaded?this.props.items.fields.map(item => (
                    <div key={item}><a className="ui_banner__items" href={this.state.apiData[item].url}>{this.state.apiData[item].description}</a></div>
                    )):<div>Waiting for data to be loaded....</div> }
                </div>
            </div>
        );
    }
}
