import { connect } from 'react-redux';
import PolicySummary from '../components/PolicySummary';
import { updateCheckingFrequency, updateEnsembler } from '../actions';

const mapStateToProps = state => {
    return {
        frequency: state.policy.CheckingFreq,
        ensembler: state.policy.Ensembler,
     };
  };

const mapDispatchToProps = dispatch => {
  return {
    updateEnsembler: event => dispatch(updateEnsembler(event.target.value)),
    updateCheckingFrequency: event => dispatch(updateCheckingFrequency(event.target.value)),
  }
}

export default connect(
    mapStateToProps,
    mapDispatchToProps
  )(PolicySummary)