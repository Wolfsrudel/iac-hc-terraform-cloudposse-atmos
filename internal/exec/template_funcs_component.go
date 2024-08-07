package exec

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/samber/lo"

	"github.com/cloudposse/atmos/pkg/utils"
)

var (
	componentFuncSyncMap = sync.Map{}
)

func componentFunc(component string, stack string) (any, error) {
	stackSlug := fmt.Sprintf("%s-%s", stack, component)

	// If the result for the component in the stack already exists in the cache, return it
	existingSections, found := componentFuncSyncMap.Load(stackSlug)
	if found && existingSections != nil {
		return existingSections, nil
	}

	sections, err := ExecuteDescribeComponent(component, stack)
	if err != nil {
		return nil, err
	}

	executable, ok := sections["command"].(string)
	if !ok {
		return nil, fmt.Errorf("the component '%s' in the stack '%s' does not have 'command' (executable) defined", component, stack)
	}

	terraformWorkspace, ok := sections["workspace"].(string)
	if !ok {
		return nil, fmt.Errorf("the component '%s' in the stack '%s' does not have Terraform/OpenTofu workspace defined", component, stack)
	}

	componentInfo, ok := sections["component_info"]
	if !ok {
		return nil, fmt.Errorf("the component '%s' in the stack '%s' does not have 'component_info' defined", component, stack)
	}

	componentInfoMap, ok := componentInfo.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the component '%s' in the stack '%s' has an invalid 'component_info' section", component, stack)
	}

	componentPath, ok := componentInfoMap["component_path"].(string)
	if !ok {
		return nil, fmt.Errorf("the component '%s' in the stack '%s' has an invalid 'component_info.component_path' section", component, stack)
	}

	tf, err := tfexec.NewTerraform(componentPath, executable)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	err = tf.Init(ctx, tfexec.Upgrade(false))
	if err != nil {
		return nil, err
	}

	err = tf.WorkspaceNew(ctx, terraformWorkspace)
	if err != nil {
		err = tf.WorkspaceSelect(ctx, terraformWorkspace)
		if err != nil {
			return nil, err
		}
	}

	outputMeta, err := tf.Output(ctx)
	if err != nil {
		return nil, err
	}

	outputMetaProcessed := lo.MapEntries(outputMeta, func(k string, v tfexec.OutputMeta) (string, any) {
		d, _ := utils.ConvertFromJSON(string(v.Value))
		return k, d
	})

	outputs := map[string]any{
		"outputs": outputMetaProcessed,
	}

	sections = lo.Assign(sections, outputs)

	// Cache the result
	componentFuncSyncMap.Store(stackSlug, sections)

	return sections, nil
}
