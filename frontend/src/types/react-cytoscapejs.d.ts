declare module 'react-cytoscapejs' {
  import type { ComponentType, CSSProperties } from 'react'
  import type { Core, ElementDefinition, StylesheetJson, LayoutOptions } from 'cytoscape'

  interface CytoscapeComponentProps {
    elements?: ElementDefinition[]
    style?: CSSProperties
    stylesheet?: StylesheetJson
    layout?: LayoutOptions
    cy?: (cy: Core) => void
  }

  const CytoscapeComponent: ComponentType<CytoscapeComponentProps>
  export default CytoscapeComponent
}

declare module 'cytoscape-dagre' {
  import type { Ext } from 'cytoscape'

  const dagre: Ext
  export default dagre
}
