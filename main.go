package main

import (
	"time"

	xdscache "github.com/kaasops/envoy-xds-controller/pkg/xds/cache"
	"github.com/kaasops/envoy-xds-controller/pkg/xds/server"
	"google.golang.org/protobuf/types/known/anypb"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	basicv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/basic_auth/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	testv3 "github.com/envoyproxy/go-control-plane/pkg/test/v3"

	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
)

const (
	xdsPort = 9000
	nodeID  = "test"

	listenerAddress = "0.0.0.0"
	listenerPort    = 8080

	routeConfigName = "test-route"

	basicAuthFilterName = "envoy.filters.http.basic_auth"
	local_service       = "local_service"
	ws_service          = "ws_service"
)

func main() {
	xDSCache := xdscache.New()
	xDSServer := server.New(xDSCache, &testv3.Callbacks{Debug: true})

	if err := fillCache(xDSCache); err != nil {
		panic(err)
	}

	xDSServer.Run(xdsPort)

	time.Sleep(1000 * time.Hour)
}

func fillCache(cache *xdscache.Cache) error {
	listener := getListener()
	if err := listener.ValidateAll(); err != nil {
		return err
	}

	routeConfig := getRouteConfiguration()
	if err := routeConfig.ValidateAll(); err != nil {
		return err
	}

	// localServiceCluster := getLocalServiceClusters()
	// if err := localServiceCluster.ValidateAll(); err != nil {
	// 	return err
	// }

	wsSecriceCluster := getWSServiceClusters()
	if err := wsSecriceCluster.ValidateAll(); err != nil {
		return err
	}

	if err := cache.Update(nodeID, listener); err != nil {
		return err
	}
	if err := cache.Update(nodeID, routeConfig); err != nil {
		return err
	}
	// if err := cache.Update(nodeID, localServiceCluster); err != nil {
	// 	return err
	// }
	if err := cache.Update(nodeID, wsSecriceCluster); err != nil {
		return err
	}

	return nil
}

/*
*

	Generate Cluster Configuration

*
*/

func getLocalServiceClusters() *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name:                 local_service,
		ClusterDiscoveryType: &clusterv3.Cluster_Type{Type: clusterv3.Cluster_STRICT_DNS},
		LbPolicy:             clusterv3.Cluster_ROUND_ROBIN,
		LoadAssignment:       getLoadAssignmentForLocalService(),
	}
}

func getLoadAssignmentForLocalService() *endpointv3.ClusterLoadAssignment {
	return &endpointv3.ClusterLoadAssignment{
		ClusterName: local_service,
		Endpoints: []*endpointv3.LocalityLbEndpoints{
			{
				LbEndpoints: []*endpointv3.LbEndpoint{
					{
						HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
							Endpoint: &endpointv3.Endpoint{
								Address: &corev3.Address{
									Address: &corev3.Address_SocketAddress{
										SocketAddress: &corev3.SocketAddress{
											Address: local_service,
											PortSpecifier: &corev3.SocketAddress_PortValue{
												PortValue: 8081,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func getWSServiceClusters() *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name:                 ws_service,
		ClusterDiscoveryType: &clusterv3.Cluster_Type{Type: clusterv3.Cluster_STRICT_DNS},
		LbPolicy:             clusterv3.Cluster_ROUND_ROBIN,
		LoadAssignment:       getLoadAssignmentForWSService(),
	}
}

func getLoadAssignmentForWSService() *endpointv3.ClusterLoadAssignment {
	return &endpointv3.ClusterLoadAssignment{
		ClusterName: local_service,
		Endpoints: []*endpointv3.LocalityLbEndpoints{
			{
				LbEndpoints: []*endpointv3.LbEndpoint{
					{
						HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
							Endpoint: &endpointv3.Endpoint{
								Address: &corev3.Address{
									Address: &corev3.Address_SocketAddress{
										SocketAddress: &corev3.SocketAddress{
											Address: "127.0.0.1",
											PortSpecifier: &corev3.SocketAddress_PortValue{
												PortValue: 8082,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

/*
*

	Generate Route Configuration

*
*/
func getRouteConfiguration() *routev3.RouteConfiguration {
	return &routev3.RouteConfiguration{
		Name: routeConfigName,
		VirtualHosts: []*routev3.VirtualHost{
			{
				Name:    routeConfigName,
				Domains: []string{"*"},
				Routes:  getRoutes(),
			},
		},
	}
}

func getRoutes() []*routev3.Route {
	action := &routev3.Route_DirectResponse{
		DirectResponse: &routev3.DirectResponseAction{
			Status: 200,
			Body: &corev3.DataSource{
				Specifier: &corev3.DataSource_InlineString{
					InlineString: "OK",
				},
			},
		},
	}

	wsAction := &routev3.Route_Route{
		Route: &routev3.RouteAction{
			ClusterSpecifier: &routev3.RouteAction_Cluster{
				Cluster: ws_service,
			},
			UpgradeConfigs: []*routev3.RouteAction_UpgradeConfig{
				{
					UpgradeType: "websocket",
				},
			},
		},
	}

	return []*routev3.Route{
		{
			Match: &routev3.RouteMatch{
				PathSpecifier: &routev3.RouteMatch_Prefix{
					Prefix: "/disable-basic-auth",
				},
			},
			Action: action,
			TypedPerFilterConfig: map[string]*anypb.Any{
				basicAuthFilterName: getDisableAuthBasic(),
			},
		},
		{
			Match: &routev3.RouteMatch{
				PathSpecifier: &routev3.RouteMatch_Prefix{
					Prefix: "/ws",
				},
			},
			Action: wsAction,
		},
		{
			Match: &routev3.RouteMatch{
				PathSpecifier: &routev3.RouteMatch_Prefix{
					Prefix: "/ws-disable-basic-auth",
				},
			},
			Action: wsAction,
			TypedPerFilterConfig: map[string]*anypb.Any{
				basicAuthFilterName: getDisableAuthBasic(),
			},
		},
		{
			Match: &routev3.RouteMatch{
				PathSpecifier: &routev3.RouteMatch_Prefix{
					Prefix: "/",
				},
			},
			Action: action,
		},
	}
}

/*
*

	Generate Listener for Cache

*
*/

func getListener() *listenerv3.Listener {
	return &listenerv3.Listener{
		Name: "test",
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address:  listenerAddress,
					Protocol: corev3.SocketAddress_TCP,
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: listenerPort,
					},
				},
			},
		},
		FilterChains: []*listenerv3.FilterChain{
			{
				Filters: getFilters(),
			},
		},
	}
}

func getFilters() []*listenerv3.Filter {
	hcmConfig, _ := anypb.New(getHCM())

	return []*listenerv3.Filter{{
		Name: wellknown.HTTPConnectionManager,
		ConfigType: &listenerv3.Filter_TypedConfig{
			TypedConfig: hcmConfig,
		},
	}}
}

func getHCM() *hcm.HttpConnectionManager {
	return &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: nodeID,
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource: &corev3.ConfigSource{
					ResourceApiVersion:    corev3.ApiVersion_V3,
					ConfigSourceSpecifier: &corev3.ConfigSource_Ads{},
				},
				RouteConfigName: routeConfigName,
			},
		},
		HttpFilters: []*hcm.HttpFilter{
			getAuthBasic(),
			getRouteFilter(),
		},
	}
}

// user:password
func getAuthBasic() *hcm.HttpFilter {
	basicAuthConfig, _ := anypb.New(&basicv3.BasicAuth{
		Users: &corev3.DataSource{
			Specifier: &corev3.DataSource_InlineString{
				InlineString: "user:{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=",
			},
		},
	})

	return &hcm.HttpFilter{
		Name: basicAuthFilterName,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: basicAuthConfig,
		},
	}
}

func getDisableAuthBasic() *anypb.Any {
	disableAuth, _ := anypb.New(&routev3.FilterConfig{Disabled: true})

	return disableAuth
}

func getRouteFilter() *hcm.HttpFilter {
	routerConfig, _ := anypb.New(&routerv3.Router{})

	return &hcm.HttpFilter{
		Name: wellknown.Router,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: routerConfig,
		},
	}
}
