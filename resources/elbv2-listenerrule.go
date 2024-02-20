package resources

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
	"github.com/sirupsen/logrus"
)

var elbv2ListenerRulePageSize int64 = 400 // AWS has a limit of 100 rules per listener

type ELBv2ListenerRule struct {
	elb      *ELBv2LoadBalancer
	rule     *elbv2.Rule
	listener *elbv2.Listener
}

func init() {
	register("ELBv2ListenerRule", ListELBv2ListenerRules)
}

func ListELBv2ListenerRules(sess *session.Session) ([]Resource, error) {
	lbs, err := ListELBv2LoadBalancers(sess)
	if err != nil {
		return nil, err
	}

	resources := make([]Resource, 0)
	for _, resLB := range lbs {
		lb := resLB.(*ELBv2LoadBalancer)

		err := lb.svc.DescribeListenersPages(
			&elbv2.DescribeListenersInput{
				LoadBalancerArn: lb.elb.LoadBalancerArn,
			},
			func(page *elbv2.DescribeListenersOutput, lastPage bool) bool {
				for _, listener := range page.Listeners {
					rules, err := lb.svc.DescribeRules(&elbv2.DescribeRulesInput{
						ListenerArn: listener.ListenerArn,
						PageSize:    &elbv2ListenerRulePageSize,
					})
					if err == nil {
						for _, rule := range rules.Rules {
							// Skip default rules as they cannot be deleted
							if rule.IsDefault == nil && *rule.IsDefault {
								continue
							}

							resources = append(resources, &ELBv2ListenerRule{
								elb:      lb,
								rule:     rule,
								listener: listener,
							})
						}
					} else {
						logrus.
							WithError(err).
							WithField("listenerArn", listener.ListenerArn).
							Error("Failed to list listener rules for listener")
					}
				}

				return !lastPage
			},
		)
		if err != nil {
			logrus.
				WithError(err).
				WithField("loadBalancerArn", lb.elb.LoadBalancerArn).
				Error("Failed to list listeners for load balancer")
		}
	}

	return resources, nil
}

func (e *ELBv2ListenerRule) Remove() error {
	_, err := e.elb.svc.DeleteRule(&elbv2.DeleteRuleInput{
		RuleArn: e.rule.RuleArn,
	})
	if err != nil {
		return err
	}

	return nil
}

func (e *ELBv2ListenerRule) Properties() types.Properties {
	properties := types.NewProperties().
		Set("ARN", e.rule.RuleArn)
	properties.Set("ListenerARN", e.listener.ListenerArn)
	properties.Set("LoadBalancer", e.elb.elb.LoadBalancerName)

	for _, tagValue := range e.elb.tags {
		properties.SetTag(tagValue.Key, tagValue.Value)
	}
	return properties
}

func (e *ELBv2ListenerRule) String() string {
	return fmt.Sprintf("%s -> %s", e.elb.String(), *e.rule.RuleArn)
}
